package files

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
)

const testUser int64 = 1

type memoryTrash struct {
	mu    sync.Mutex
	items map[string]TrashItem
}

func (m *memoryTrash) AddTrashItem(_ context.Context, item TrashItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[item.ID] = item
	return nil
}

func (m *memoryTrash) TrashItems(_ context.Context, userID int64) ([]TrashItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []TrashItem
	for _, item := range m.items {
		if item.UserID == userID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (m *memoryTrash) TrashItem(_ context.Context, userID int64, id string) (TrashItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[id]
	if !ok || item.UserID != userID {
		return TrashItem{}, ErrTrashItemNotFound
	}
	return item, nil
}

func (m *memoryTrash) DeleteTrashItem(_ context.Context, _ int64, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.items, id)
	return nil
}

func (m *memoryTrash) ExpiredTrashItems(_ context.Context, before time.Time) ([]TrashItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []TrashItem
	for _, item := range m.items {
		if item.TrashedAt.Before(before) {
			out = append(out, item)
		}
	}
	return out, nil
}

type recordingObserver struct {
	moved   [][2]string
	removed []string
}

func (r *recordingObserver) PathMoved(_ context.Context, _ int64, from, to Path) error {
	r.moved = append(r.moved, [2]string{from.String(), to.String()})
	return nil
}

func (r *recordingObserver) PathRemoved(_ context.Context, _ int64, p Path) error {
	r.removed = append(r.removed, p.String())
	return nil
}

type fixedQuota int64

func (q fixedQuota) QuotaBytes(context.Context) (int64, error) { return int64(q), nil }

type fixture struct {
	svc      *Service
	dir      string
	trash    *memoryTrash
	observer *recordingObserver
	now      time.Time
}

func newFixture(t *testing.T, quota int64) *fixture {
	t.Helper()
	dir := t.TempDir()
	vol, err := OpenVolume(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vol.Close() })
	if err := vol.EnsureUser(testUser); err != nil {
		t.Fatal(err)
	}
	f := &fixture{
		dir:      dir,
		trash:    &memoryTrash{items: map[string]TrashItem{}},
		observer: &recordingObserver{},
		now:      time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC),
	}
	var seq int
	f.svc = NewService(Config{
		Volume:   vol,
		Trash:    f.trash,
		Observer: f.observer,
		Quota:    fixedQuota(quota),
		Clock:    func() time.Time { return f.now },
		NewID:    func() string { seq++; return "id" + strconv.Itoa(seq) },
		UsageTTL: time.Minute,
	})
	return f
}

func (f *fixture) write(t *testing.T, rel string, size int) {
	t.Helper()
	full := filepath.Join(f.dir, "users", "1", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, bytes.Repeat([]byte("x"), size), 0o640); err != nil {
		t.Fatal(err)
	}
}

func mustPath(t *testing.T, raw string) Path {
	t.Helper()
	p, err := ParsePath(raw)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func names(entries []Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestListSortsFoldersFirstAndHidesLegacyStaging(t *testing.T) {
	f := newFixture(t, 0)
	ctx := context.Background()
	f.write(t, "b.txt", 1)
	f.write(t, "A.txt", 1)
	f.write(t, "zeta/file", 1)
	f.write(t, ".tus/abc.bin", 1)

	entries, err := f.svc.List(ctx, testUser, Root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"zeta", "A.txt", "b.txt"}
	if got := names(entries); !slices.Equal(got, want) {
		t.Fatalf("List() = %v, want %v", got, want)
	}
	if _, err := f.svc.List(ctx, testUser, mustPath(t, ".tus")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("listing reserved dir error = %v", err)
	}
	if _, err := f.svc.List(ctx, testUser, mustPath(t, "b.txt")); !errors.Is(err, ErrNotDirectory) {
		t.Fatalf("listing a file error = %v", err)
	}
}

func TestCreateFolder(t *testing.T) {
	f := newFixture(t, 0)
	ctx := context.Background()

	entry, err := f.svc.CreateFolder(ctx, testUser, mustPath(t, "docs"))
	if err != nil || entry.Kind != KindDir {
		t.Fatalf("CreateFolder() = %+v, %v", entry, err)
	}
	if _, err := f.svc.CreateFolder(ctx, testUser, mustPath(t, "docs")); !errors.Is(err, ErrExists) {
		t.Fatalf("duplicate folder error = %v", err)
	}
	if _, err := f.svc.CreateFolder(ctx, testUser, mustPath(t, "missing/child")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing parent error = %v", err)
	}
	if _, err := f.svc.CreateFolder(ctx, testUser, Root); !errors.Is(err, ErrRootOperation) {
		t.Fatalf("root error = %v", err)
	}
}

func TestRenameChangingOnlyCaseKeepsOtherFiles(t *testing.T) {
	f := newFixture(t, 0)
	ctx := context.Background()
	f.write(t, "Report.txt", 3)
	if entry, err := f.svc.Rename(ctx, testUser, mustPath(t, "Report.txt"), "REPORT.txt"); err != nil || entry.Name() != "REPORT.txt" {
		t.Fatalf("case-only rename = %+v, %v", entry, err)
	}
	f.write(t, "report.txt", 5)
	upper, uerr := os.Stat(filepath.Join(f.dir, "users", "1", "REPORT.txt"))
	lower, lerr := os.Stat(filepath.Join(f.dir, "users", "1", "report.txt"))
	if uerr != nil || lerr != nil || os.SameFile(upper, lower) {
		t.Skip("file system is case-insensitive")
	}
	if _, err := f.svc.Rename(ctx, testUser, mustPath(t, "REPORT.txt"), "report.txt"); !errors.Is(err, ErrExists) {
		t.Fatalf("rename onto a file differing in case error = %v", err)
	}
	if info, err := os.Stat(filepath.Join(f.dir, "users", "1", "report.txt")); err != nil || info.Size() != 5 {
		t.Fatalf("existing file replaced: %v, %v", info, err)
	}
}

func TestRenameAndMove(t *testing.T) {
	f := newFixture(t, 0)
	ctx := context.Background()
	f.write(t, "a/one.txt", 3)
	f.write(t, "a/two.txt", 3)
	f.write(t, "b/two.txt", 3)

	entry, err := f.svc.Rename(ctx, testUser, mustPath(t, "a/one.txt"), "uno.txt")
	if err != nil || entry.Path.String() != "a/uno.txt" {
		t.Fatalf("Rename() = %+v, %v", entry, err)
	}
	if _, err := f.svc.Rename(ctx, testUser, mustPath(t, "a/uno.txt"), "two.txt"); !errors.Is(err, ErrExists) {
		t.Fatalf("rename onto existing error = %v", err)
	}
	if _, err := f.svc.Rename(ctx, testUser, mustPath(t, "a/uno.txt"), "../x"); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("rename traversal error = %v", err)
	}
	if err := f.svc.Move(ctx, testUser, []Path{mustPath(t, "a")}, mustPath(t, "a")); !errors.Is(err, ErrMoveIntoSelf) {
		t.Fatalf("move into self error = %v", err)
	}
	if err := f.svc.Move(ctx, testUser, []Path{mustPath(t, "a/two.txt")}, mustPath(t, "b")); !errors.Is(err, ErrExists) {
		t.Fatalf("move conflict error = %v", err)
	}
	if err := f.svc.Move(ctx, testUser, []Path{mustPath(t, "a/uno.txt")}, mustPath(t, "b")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Stat(ctx, testUser, mustPath(t, "b/uno.txt")); err != nil {
		t.Fatalf("moved file missing: %v", err)
	}
	wantMoves := [][2]string{{"a/one.txt", "a/uno.txt"}, {"a/uno.txt", "b/uno.txt"}}
	if !slices.Equal(f.observer.moved, wantMoves) {
		t.Fatalf("observer moves = %v, want %v", f.observer.moved, wantMoves)
	}
}

func TestTrashRestoreAndPurge(t *testing.T) {
	f := newFixture(t, 0)
	ctx := context.Background()
	f.write(t, "keep/a.txt", 10)
	f.write(t, "keep/b.txt", 5)

	if err := f.svc.Trash(ctx, testUser, []Path{mustPath(t, "keep"), mustPath(t, "keep/a.txt")}); err != nil {
		t.Fatal(err)
	}
	items, _ := f.svc.TrashItems(ctx, testUser)
	if len(items) != 1 || items[0].Size != 15 || items[0].Kind != KindDir {
		t.Fatalf("trash items = %+v", items)
	}
	usage, err := f.svc.Usage(ctx, testUser)
	if err != nil || usage.FilesBytes != 0 || usage.TrashBytes != 15 {
		t.Fatalf("usage after trash = %+v, %v", usage, err)
	}
	f.write(t, "keep/new.txt", 1)
	restored, err := f.svc.Restore(ctx, testUser, items[0].ID)
	if err != nil || restored.String() != "keep (1)" {
		t.Fatalf("Restore() = %q, %v", restored.String(), err)
	}

	if err := f.svc.Trash(ctx, testUser, []Path{mustPath(t, "keep")}); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(31 * 24 * time.Hour)
	purged, err := f.svc.PurgeTrash(ctx, 30*24*time.Hour)
	if err != nil || purged != 1 {
		t.Fatalf("PurgeTrash() = %d, %v", purged, err)
	}
	if _, err := os.Stat(filepath.Join(f.dir, ".system", "trash", "1", "id2")); !os.IsNotExist(err) {
		t.Fatalf("purged item still on disk: %v", err)
	}
	if !slices.Equal(f.observer.removed, []string{"keep", "keep"}) {
		t.Fatalf("observer removed = %v", f.observer.removed)
	}
}

func TestPublishPicksUniqueName(t *testing.T) {
	f := newFixture(t, 0)
	ctx := context.Background()
	f.write(t, "in/report.pdf", 1)
	for _, id := range []string{"u1", "u2"} {
		staged, err := f.svc.volume.CreateStaging(testUser, id)
		if err != nil {
			t.Fatal(err)
		}
		staged.Close()
	}
	first, err := f.svc.Publish(ctx, testUser, "u1", Publication{Dir: mustPath(t, "in"), Name: "report.pdf"})
	if err != nil || first.String() != "in/report (1).pdf" {
		t.Fatalf("Publish() = %q, %v", first.String(), err)
	}
	second, err := f.svc.Publish(ctx, testUser, "u2", Publication{Dir: mustPath(t, "in"), Name: "report.pdf"})
	if err != nil || second.String() != "in/report (2).pdf" {
		t.Fatalf("Publish() = %q, %v", second.String(), err)
	}
}

func TestSearchFindsNestedMatches(t *testing.T) {
	f := newFixture(t, 0)
	f.write(t, "x/Report-2026.pdf", 1)
	f.write(t, "x/y/report.txt", 1)
	f.write(t, "other.txt", 1)

	found, err := f.svc.Search(context.Background(), testUser, Root, "REPORT", 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(found); !slices.Equal(got, []string{"Report-2026.pdf", "report.txt"}) {
		t.Fatalf("Search() = %v", got)
	}
}

func TestUsageAndCapacity(t *testing.T) {
	f := newFixture(t, 100)
	ctx := context.Background()
	f.write(t, "photos/a.jpg", 40)
	f.write(t, "docs/b.pdf", 20)
	f.write(t, "root.bin", 5)

	usage, err := f.svc.Usage(ctx, testUser)
	if err != nil {
		t.Fatal(err)
	}
	if usage.FilesBytes != 65 || usage.FileCount != 3 {
		t.Fatalf("usage = %+v", usage)
	}
	if usage.ByCategory[CategoryImage] != 40 || usage.ByCategory[CategoryDocument] != 20 {
		t.Fatalf("by category = %v", usage.ByCategory)
	}
	if usage.ByFolder[0] != (FolderUsage{Name: "photos", Bytes: 40}) {
		t.Fatalf("by folder = %v", usage.ByFolder)
	}
	if usage.Largest[0].Path.String() != "photos/a.jpg" {
		t.Fatalf("largest = %v", usage.Largest)
	}
	if err := f.svc.EnsureCapacity(ctx, testUser, 30, 0); err != nil {
		t.Fatalf("EnsureCapacity within quota: %v", err)
	}
	if err := f.svc.EnsureCapacity(ctx, testUser, 30, 10); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("EnsureCapacity over quota error = %v", err)
	}
}

func TestUsageCacheInvalidatesOnMutation(t *testing.T) {
	f := newFixture(t, 0)
	ctx := context.Background()
	f.write(t, "a.txt", 10)
	if u, _ := f.svc.Usage(ctx, testUser); u.FilesBytes != 10 {
		t.Fatalf("initial usage = %d", u.FilesBytes)
	}
	f.write(t, "b.txt", 10)
	if u, _ := f.svc.Usage(ctx, testUser); u.FilesBytes != 10 {
		t.Fatalf("cached usage = %d", u.FilesBytes)
	}
	if err := f.svc.Trash(ctx, testUser, []Path{mustPath(t, "a.txt")}); err != nil {
		t.Fatal(err)
	}
	if u, _ := f.svc.Usage(ctx, testUser); u.FilesBytes != 10 || u.TrashBytes != 10 {
		t.Fatalf("usage after invalidation = %+v", u)
	}
}

func TestWriteArchive(t *testing.T) {
	f := newFixture(t, 0)
	f.write(t, "p/album/one.jpg", 4)
	f.write(t, "p/album/sub/two.txt", 4)
	f.write(t, "p/notes.txt", 2)

	var buf bytes.Buffer
	err := f.svc.WriteArchive(context.Background(), testUser, []Path{mustPath(t, "p/album"), mustPath(t, "p/notes.txt")}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, zf := range zr.File {
		got = append(got, zf.Name)
		rc, _ := zf.Open()
		io.Copy(io.Discard, rc)
		rc.Close()
	}
	slices.Sort(got)
	want := []string{"album/one.jpg", "album/sub/two.txt", "notes.txt"}
	if !slices.Equal(got, want) {
		t.Fatalf("archive entries = %v, want %v", got, want)
	}
}

func TestSymlinkEscapeIsRejected(t *testing.T) {
	f := newFixture(t, 0)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(f.dir, "users", "1", "escape.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.Open(context.Background(), testUser, mustPath(t, "escape.txt")); err == nil {
		t.Fatal("Open() followed a symlink outside the volume")
	}
}
