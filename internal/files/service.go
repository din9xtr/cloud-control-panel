package files

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxUniqueAttempts = 1000

type TrashStore interface {
	AddTrashItem(ctx context.Context, item TrashItem) error
	TrashItems(ctx context.Context, userID int64) ([]TrashItem, error)
	TrashItem(ctx context.Context, userID int64, id string) (TrashItem, error)
	DeleteTrashItem(ctx context.Context, userID int64, id string) error
	ExpiredTrashItems(ctx context.Context, before time.Time) ([]TrashItem, error)
}

type PathObserver interface {
	PathMoved(ctx context.Context, userID int64, from, to Path) error
	PathRemoved(ctx context.Context, userID int64, p Path) error
}

type QuotaProvider interface {
	QuotaBytes(ctx context.Context) (int64, error)
}

type Config struct {
	Volume   *Volume
	Trash    TrashStore
	Observer PathObserver
	Quota    QuotaProvider
	Clock    func() time.Time
	NewID    func() string
	UsageTTL time.Duration
}

type Service struct {
	volume   *Volume
	trash    TrashStore
	observer PathObserver
	quota    QuotaProvider
	clock    func() time.Time
	newID    func() string
	usage    *usageCache
	mu       sync.Mutex
}

func NewService(cfg Config) *Service {
	s := &Service{
		volume:   cfg.Volume,
		trash:    cfg.Trash,
		observer: cfg.Observer,
		quota:    cfg.Quota,
		clock:    cfg.Clock,
		newID:    cfg.NewID,
	}
	s.usage = newUsageCache(cfg.UsageTTL, cfg.Clock, s.computeUsage)
	return s
}

func (s *Service) List(_ context.Context, userID int64, dir Path) ([]Entry, error) {
	if isReserved(dir) {
		return nil, ErrNotFound
	}
	f, err := s.volume.root.Open(userPath(userID, dir))
	if err != nil {
		return nil, mapFSError(err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, mapFSError(err)
	}
	if !info.IsDir() {
		return nil, ErrNotDirectory
	}
	dirEntries, err := f.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	entries := make([]Entry, 0, len(dirEntries))
	for _, d := range dirEntries {
		p, ok := s.visibleChild(dir, d)
		if !ok {
			continue
		}
		info, err := d.Info()
		if err != nil {
			continue
		}
		entries = append(entries, newEntry(p, info))
	}
	sortEntries(entries)
	return entries, nil
}

func (s *Service) Stat(_ context.Context, userID int64, p Path) (Entry, error) {
	if isReserved(p) {
		return Entry{}, ErrNotFound
	}
	info, err := s.volume.root.Stat(userPath(userID, p))
	if err != nil {
		return Entry{}, mapFSError(err)
	}
	return newEntry(p, info), nil
}

func (s *Service) CreateFolder(ctx context.Context, userID int64, p Path) (Entry, error) {
	if err := checkMutable(p); err != nil {
		return Entry{}, err
	}
	if err := ValidateNewName(p.Name()); err != nil {
		return Entry{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireDir(userID, p.Parent()); err != nil {
		return Entry{}, err
	}
	if err := s.volume.root.Mkdir(userPath(userID, p), dirPerm); err != nil {
		return Entry{}, mapFSError(err)
	}
	return s.Stat(ctx, userID, p)
}

func (s *Service) Rename(ctx context.Context, userID int64, p Path, newName string) (Entry, error) {
	if err := checkMutable(p); err != nil {
		return Entry{}, err
	}
	if err := ValidateNewName(newName); err != nil {
		return Entry{}, err
	}
	target, err := p.Parent().Join(newName)
	if err != nil {
		return Entry{}, err
	}
	if err := checkMutable(target); err != nil {
		return Entry{}, err
	}
	if target == p {
		return s.Stat(ctx, userID, p)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	source, err := s.volume.root.Lstat(userPath(userID, p))
	if err != nil {
		return Entry{}, mapFSError(err)
	}
	if s.occupiedByOther(userID, source, target) {
		return Entry{}, ErrExists
	}
	if err := s.volume.root.Rename(userPath(userID, p), userPath(userID, target)); err != nil {
		return Entry{}, mapFSError(err)
	}
	s.usage.invalidate(userID)
	if err := s.observer.PathMoved(ctx, userID, p, target); err != nil {
		return Entry{}, fmt.Errorf("update links: %w", err)
	}
	return s.Stat(ctx, userID, target)
}

func (s *Service) Move(ctx context.Context, userID int64, paths []Path, dest Path) error {
	sources := topLevel(paths)
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireDir(userID, dest); err != nil {
		return err
	}
	moves := make([][2]Path, 0, len(sources))
	for _, src := range sources {
		if err := checkMutable(src); err != nil {
			return err
		}
		if src.Parent() == dest {
			continue
		}
		if src.Contains(dest) {
			return ErrMoveIntoSelf
		}
		if _, err := s.volume.root.Lstat(userPath(userID, src)); err != nil {
			return mapFSError(err)
		}
		target, err := dest.Join(src.Name())
		if err != nil {
			return err
		}
		if s.exists(userID, target) {
			return ErrExists
		}
		moves = append(moves, [2]Path{src, target})
	}
	if len(moves) > 0 {
		s.usage.invalidate(userID)
	}
	for _, m := range moves {
		if err := s.volume.root.Rename(userPath(userID, m[0]), userPath(userID, m[1])); err != nil {
			return mapFSError(err)
		}
		if err := s.observer.PathMoved(ctx, userID, m[0], m[1]); err != nil {
			return fmt.Errorf("update links: %w", err)
		}
	}
	return nil
}

func (s *Service) Open(_ context.Context, userID int64, p Path) (*os.File, Entry, error) {
	if isReserved(p) {
		return nil, Entry{}, ErrNotFound
	}
	f, err := s.volume.root.Open(userPath(userID, p))
	if err != nil {
		return nil, Entry{}, mapFSError(err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, Entry{}, mapFSError(err)
	}
	if info.IsDir() {
		_ = f.Close()
		return nil, Entry{}, ErrIsDirectory
	}
	return f, newEntry(p, info), nil
}

func (s *Service) PrepareUser(_ context.Context, userID int64) error {
	return s.volume.EnsureUser(userID)
}

func (s *Service) RequireFolder(_ context.Context, userID int64, dir Path) error {
	return s.requireDir(userID, dir)
}

func (s *Service) Search(ctx context.Context, userID int64, dir Path, query string, limit int) ([]Entry, error) {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return nil, nil
	}
	fsys, err := s.volume.userFS(userID)
	if err != nil {
		return nil, err
	}
	var found []Entry
	err = fs.WalkDir(fsys, dir.fsPath(), func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			if name == dir.fsPath() {
				return mapFSError(err)
			}
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if name == dir.fsPath() {
			return nil
		}
		p, perr := ParsePath(name)
		if perr != nil || isReserved(p) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.Contains(strings.ToLower(d.Name()), needle) {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		found = append(found, newEntry(p, info))
		if len(found) >= limit {
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortEntries(found)
	return found, nil
}

func (s *Service) visibleChild(dir Path, d fs.DirEntry) (Path, bool) {
	if dir.IsRoot() && d.Name() == legacyTus {
		return Path{}, false
	}
	if !d.IsDir() && !d.Type().IsRegular() {
		return Path{}, false
	}
	p, err := dir.Join(d.Name())
	if err != nil {
		return Path{}, false
	}
	return p, true
}

func (s *Service) requireDir(userID int64, dir Path) error {
	if isReserved(dir) {
		return ErrNotFound
	}
	info, err := s.volume.root.Stat(userPath(userID, dir))
	if err != nil {
		return mapFSError(err)
	}
	if !info.IsDir() {
		return ErrNotDirectory
	}
	return nil
}

func (s *Service) exists(userID int64, p Path) bool {
	_, err := s.volume.root.Lstat(userPath(userID, p))
	return !errors.Is(err, fs.ErrNotExist)
}

func (s *Service) occupiedByOther(userID int64, source fs.FileInfo, target Path) bool {
	existing, err := s.volume.root.Lstat(userPath(userID, target))
	if errors.Is(err, fs.ErrNotExist) {
		return false
	}
	return err != nil || !os.SameFile(source, existing)
}

func (s *Service) uniquePath(userID int64, dir Path, name string) (Path, error) {
	candidate, err := dir.Join(name)
	if err != nil {
		return Path{}, err
	}
	if isReserved(candidate) {
		return Path{}, ErrInvalidName
	}
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; s.exists(userID, candidate); i++ {
		if i > maxUniqueAttempts {
			return Path{}, ErrExists
		}
		candidate, err = dir.Join(base + " (" + strconv.Itoa(i) + ")" + ext)
		if err != nil {
			return Path{}, err
		}
	}
	return candidate, nil
}

func checkMutable(p Path) error {
	if p.IsRoot() {
		return ErrRootOperation
	}
	if isReserved(p) {
		return ErrInvalidName
	}
	return nil
}

func topLevel(paths []Path) []Path {
	sorted := slices.Clone(paths)
	slices.SortFunc(sorted, func(a, b Path) int { return cmp.Compare(a.String(), b.String()) })
	sorted = slices.Compact(sorted)
	out := make([]Path, 0, len(sorted))
	for _, p := range sorted {
		if len(out) > 0 && out[len(out)-1].Contains(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func sortEntries(entries []Entry) {
	slices.SortFunc(entries, func(a, b Entry) int {
		if a.Kind != b.Kind {
			if a.Kind == KindDir {
				return -1
			}
			return 1
		}
		if c := cmp.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name())); c != 0 {
			return c
		}
		return cmp.Compare(a.Path.String(), b.Path.String())
	})
}
