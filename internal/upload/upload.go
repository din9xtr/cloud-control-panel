package upload

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/din9xtr/cloud-control-panel/internal/files"
)

const (
	MaxActive  = 200
	MaxSize    = int64(4) << 40
	idBytes    = 16
	bufferSize = 1 << 20
)

var (
	ErrNotFound       = errors.New("upload not found")
	ErrOffsetMismatch = errors.New("upload offset mismatch")
	ErrLocked         = errors.New("upload is busy")
	ErrTooLarge       = errors.New("upload exceeds declared length")
	ErrInvalidSize    = errors.New("invalid upload length")
	ErrTooManyUploads = errors.New("too many unfinished uploads")
)

type Upload struct {
	ShareID   string
	ID        string
	UserID    int64
	Size      int64
	Offset    int64
	Dir       files.Path
	Name      string
	CreatedAt time.Time
	ExpiresAt time.Time
	Result    files.Path
	Ready     bool
}

func (u Upload) Done() bool {
	return !u.Result.IsRoot()
}

type Store interface {
	CreateUpload(ctx context.Context, u Upload) error
	Upload(ctx context.Context, userID int64, id string) (Upload, error)
	UpdateOffset(ctx context.Context, id string, offset int64, expiresAt time.Time) error
	ReadyUpload(ctx context.Context, id string, expiresAt time.Time) error
	CompleteUpload(ctx context.Context, id string, result files.Path) error
	DeleteUpload(ctx context.Context, id string) error
	ReservedBytes(ctx context.Context, userID int64) (int64, error)
	ActiveUploads(ctx context.Context, userID int64) (int, error)
	ExpiredUploads(ctx context.Context, now time.Time) ([]Upload, error)
}

type Staging interface {
	CreateStaging(userID int64, id string) (*os.File, error)
	OpenStaging(userID int64, id string) (*os.File, error)
	RemoveStaging(userID int64, id string) error
}

type Library interface {
	RequireFolder(ctx context.Context, userID int64, dir files.Path) error
	EnsureCapacity(ctx context.Context, userID, need, reserved int64) error
	Publish(ctx context.Context, userID int64, stagingID string, in files.Publication) (files.Path, error)
}

type Config struct {
	Store   Store
	Staging Staging
	Library Library
	Clock   func() time.Time
	TTL     time.Duration
}

type Service struct {
	store    Store
	staging  Staging
	library  Library
	clock    func() time.Time
	ttl      time.Duration
	createMu sync.Mutex
	locks    keyedLock
	buffers  sync.Pool
}

func NewService(cfg Config) *Service {
	return &Service{
		store:   cfg.Store,
		staging: cfg.Staging,
		library: cfg.Library,
		clock:   cfg.Clock,
		ttl:     cfg.TTL,
		locks:   keyedLock{held: make(map[string]struct{})},
		buffers: sync.Pool{New: func() any { b := make([]byte, bufferSize); return &b }},
	}
}

type CreateInput struct {
	ShareID string
	Size    int64
	Dir     files.Path
	Name    string
}

func (s *Service) Create(ctx context.Context, userID int64, in CreateInput) (Upload, error) {
	if in.Size < 0 || in.Size > MaxSize {
		return Upload{}, ErrInvalidSize
	}
	if err := files.ValidateNewName(in.Name); err != nil {
		return Upload{}, err
	}
	if err := s.library.RequireFolder(ctx, userID, in.Dir); err != nil {
		return Upload{}, err
	}

	s.createMu.Lock()
	defer s.createMu.Unlock()
	active, err := s.store.ActiveUploads(ctx, userID)
	if err != nil {
		return Upload{}, fmt.Errorf("active uploads: %w", err)
	}
	if active >= MaxActive {
		return Upload{}, ErrTooManyUploads
	}
	reserved, err := s.store.ReservedBytes(ctx, userID)
	if err != nil {
		return Upload{}, fmt.Errorf("reserved bytes: %w", err)
	}
	if err := s.library.EnsureCapacity(ctx, userID, in.Size, reserved); err != nil {
		return Upload{}, err
	}
	id, err := newID()
	if err != nil {
		return Upload{}, err
	}
	f, err := s.staging.CreateStaging(userID, id)
	if err != nil {
		return Upload{}, fmt.Errorf("create staging: %w", err)
	}
	if err := f.Close(); err != nil {
		return Upload{}, errors.Join(fmt.Errorf("close staging: %w", err), s.staging.RemoveStaging(userID, id))
	}
	now := s.clock().UTC()
	u := Upload{ID: id, UserID: userID, Size: in.Size, Dir: in.Dir, Name: in.Name, CreatedAt: now, ExpiresAt: now.Add(s.ttl), ShareID: in.ShareID}
	if err := s.store.CreateUpload(ctx, u); err != nil {
		return Upload{}, errors.Join(fmt.Errorf("store upload: %w", err), s.staging.RemoveStaging(userID, id))
	}
	if u.Size == 0 {
		if err := s.store.ReadyUpload(ctx, u.ID, u.ExpiresAt); err != nil {
			return u, err
		}
		u.Ready = true
		return s.publishLocked(ctx, u)
	}
	return u, nil
}

func (s *Service) Reserved(ctx context.Context, userID int64) (int64, error) {
	return s.store.ReservedBytes(ctx, userID)
}

func (s *Service) Get(ctx context.Context, userID int64, id string) (Upload, error) {
	return s.store.Upload(ctx, userID, id)
}

func (s *Service) Status(ctx context.Context, userID int64, id string) (Upload, error) {
	if !s.locks.tryLock(id) {
		return Upload{}, ErrLocked
	}
	defer s.locks.unlock(id)
	u, err := s.store.Upload(ctx, userID, id)
	if err != nil {
		return Upload{}, err
	}
	if u.Ready && !u.Done() {
		return s.publish(ctx, u)
	}
	return u, nil
}

func (s *Service) Write(ctx context.Context, userID int64, id string, offset int64, body io.Reader) (Upload, error) {
	if !s.locks.tryLock(id) {
		return Upload{}, ErrLocked
	}
	defer s.locks.unlock(id)

	u, err := s.store.Upload(ctx, userID, id)
	if err != nil {
		return Upload{}, err
	}
	if offset != u.Offset {
		return u, ErrOffsetMismatch
	}
	if u.Done() {
		return u, nil
	}
	if u.Ready {
		return s.publish(ctx, u)
	}
	written, copyErr := s.append(u, body)
	u.ExpiresAt = s.clock().UTC().Add(s.ttl)
	if copyErr == nil && u.Offset+written == u.Size {
		if err := s.store.ReadyUpload(ctx, id, u.ExpiresAt); err != nil {
			return u, fmt.Errorf("save ready upload: %w", err)
		}
		u.Offset, u.Ready = u.Size, true
		return s.publish(ctx, u)
	}
	u.Offset += written
	if err := s.store.UpdateOffset(ctx, id, u.Offset, u.ExpiresAt); err != nil {
		return u, errors.Join(copyErr, fmt.Errorf("save offset: %w", err))
	}
	if copyErr != nil {
		return u, copyErr
	}
	return u, nil
}

func (s *Service) Terminate(ctx context.Context, userID int64, id string) error {
	if !s.locks.tryLock(id) {
		return ErrLocked
	}
	defer s.locks.unlock(id)
	if _, err := s.store.Upload(ctx, userID, id); err != nil {
		return err
	}
	return s.discard(ctx, userID, id)
}

func (s *Service) PurgeExpired(ctx context.Context) (int, error) {
	expired, err := s.store.ExpiredUploads(ctx, s.clock())
	if err != nil {
		return 0, err
	}
	purged := 0
	for _, u := range expired {
		if !s.locks.tryLock(u.ID) {
			continue
		}
		current, err := s.store.Upload(ctx, u.UserID, u.ID)
		if err == nil && !s.clock().Before(current.ExpiresAt) {
			err = s.discard(ctx, u.UserID, u.ID)
		} else {
			s.locks.unlock(u.ID)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return purged, err
			}
			continue
		}
		s.locks.unlock(u.ID)
		if err != nil {
			return purged, err
		}
		purged++
	}
	return purged, nil
}

func (s *Service) append(u Upload, body io.Reader) (int64, error) {
	f, err := s.staging.OpenStaging(u.UserID, u.ID)
	if err != nil {
		return 0, fmt.Errorf("open staging: %w", err)
	}
	info, err := f.Stat()
	if err != nil || info.Size() < u.Offset {
		_ = f.Close()
		return 0, errors.Join(errors.New("staging is shorter than acknowledged offset"), err)
	}
	if err := f.Truncate(u.Offset); err != nil {
		_ = f.Close()
		return 0, fmt.Errorf("truncate staging: %w", err)
	}
	buf := s.buffers.Get().(*[]byte)
	defer s.buffers.Put(buf)

	remaining := u.Size - u.Offset
	written, copyErr := io.CopyBuffer(io.NewOffsetWriter(f, u.Offset), io.LimitReader(body, remaining), *buf)
	if copyErr == nil && written == remaining {
		n, err := body.Read((*buf)[:1])
		switch {
		case n > 0:
			copyErr = ErrTooLarge
		case err != nil && !errors.Is(err, io.EOF):
			copyErr = err
		case err == nil:
			copyErr = io.ErrNoProgress
		}
	}
	if copyErr != nil && written == remaining {
		written = 0
	}
	if err := f.Sync(); err != nil {
		written = 0
		copyErr = errors.Join(copyErr, fmt.Errorf("sync staging: %w", err))
	}
	if err := f.Close(); err != nil && copyErr == nil {
		written = 0
		copyErr = fmt.Errorf("close staging: %w", err)
	}
	return written, copyErr
}

func (s *Service) publish(ctx context.Context, u Upload) (Upload, error) {
	s.createMu.Lock()
	defer s.createMu.Unlock()
	return s.publishLocked(ctx, u)
}

func (s *Service) publishLocked(ctx context.Context, u Upload) (Upload, error) {
	reserved, err := s.store.ReservedBytes(ctx, u.UserID)
	if err != nil {
		return u, fmt.Errorf("reserved bytes: %w", err)
	}
	result, err := s.library.Publish(ctx, u.UserID, u.ID, files.Publication{Dir: u.Dir, Name: u.Name, Size: u.Size, Reserved: max(0, reserved-u.Size)})
	if err != nil {
		return u, fmt.Errorf("publish upload: %w", err)
	}
	u.Result = result
	if err := s.store.CompleteUpload(ctx, u.ID, result); err != nil {
		return u, fmt.Errorf("complete upload: %w", err)
	}
	if err := s.staging.RemoveStaging(u.UserID, u.ID); err != nil {
		return u, fmt.Errorf("clean completed upload: %w", err)
	}
	return u, nil
}

func (s *Service) discard(ctx context.Context, userID int64, id string) error {
	if err := s.staging.RemoveStaging(userID, id); err != nil {
		return err
	}
	return s.store.DeleteUpload(ctx, id)
}

func newID() (string, error) {
	buf := make([]byte, idBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("upload id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

type keyedLock struct {
	mu   sync.Mutex
	held map[string]struct{}
}

func (k *keyedLock) tryLock(key string) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	if _, busy := k.held[key]; busy {
		return false
	}
	k.held[key] = struct{}{}
	return true
}

func (k *keyedLock) unlock(key string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.held, key)
}
