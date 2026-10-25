package upload

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/din9xtr/cloud-control-panel/internal/files"
)

type memoryStore struct {
	mu      sync.Mutex
	uploads map[string]Upload
}

func (m *memoryStore) CreateUpload(_ context.Context, u Upload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.uploads[u.ID] = u
	return nil
}

func (m *memoryStore) Upload(_ context.Context, userID int64, id string) (Upload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.uploads[id]
	if !ok || u.UserID != userID {
		return Upload{}, ErrNotFound
	}
	return u, nil
}

func (m *memoryStore) UpdateOffset(_ context.Context, id string, offset int64, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.uploads[id]
	u.Offset, u.ExpiresAt = offset, expiresAt
	m.uploads[id] = u
	return nil
}

func (m *memoryStore) DeleteUpload(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.uploads[id]; !ok {
		return ErrNotFound
	}
	delete(m.uploads, id)
	return nil
}

func (m *memoryStore) ReservedBytes(_ context.Context, userID int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var total int64
	for _, u := range m.uploads {
		if u.UserID == userID {
			total += u.Size
		}
	}
	return total, nil
}

func (m *memoryStore) ActiveUploads(_ context.Context, userID int64) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, u := range m.uploads {
		if u.UserID == userID {
			n++
		}
	}
	return n, nil
}

func (m *memoryStore) ExpiredUploads(_ context.Context, now time.Time) ([]Upload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Upload
	for _, u := range m.uploads {
		if !now.Before(u.ExpiresAt) {
			out = append(out, u)
		}
	}
	return out, nil
}

type noTrash struct{}

func (noTrash) AddTrashItem(context.Context, files.TrashItem) error { return nil }
func (noTrash) TrashItems(context.Context, int64) ([]files.TrashItem, error) {
	return nil, nil
}

func (noTrash) TrashItem(context.Context, int64, string) (files.TrashItem, error) {
	return files.TrashItem{}, files.ErrTrashItemNotFound
}
func (noTrash) DeleteTrashItem(context.Context, int64, string) error { return nil }
func (noTrash) ExpiredTrashItems(context.Context, time.Time) ([]files.TrashItem, error) {
	return nil, nil
}

type noObserver struct{}

func (noObserver) PathMoved(context.Context, int64, files.Path, files.Path) error { return nil }
func (noObserver) PathRemoved(context.Context, int64, files.Path) error           { return nil }

type quota int64

func (q quota) QuotaBytes(context.Context) (int64, error) { return int64(q), nil }

type env struct {
	svc   *Service
	store *memoryStore
	dir   string
	now   time.Time
}

func newEnv(t *testing.T, q int64) *env {
	t.Helper()
	dir := t.TempDir()
	vol, err := files.OpenVolume(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { vol.Close() })
	if err := vol.EnsureUser(1); err != nil {
		t.Fatal(err)
	}
	e := &env{store: &memoryStore{uploads: map[string]Upload{}}, dir: dir, now: time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)}
	clock := func() time.Time { return e.now }
	lib := files.NewService(files.Config{
		Volume: vol, Trash: noTrash{}, Observer: noObserver{}, Quota: quota(q),
		Clock: clock, NewID: func() string { return "x" }, UsageTTL: time.Minute,
	})
	e.svc = NewService(Config{Store: e.store, Staging: vol, Library: lib, Clock: clock, TTL: 24 * time.Hour})
	return e
}

type failingReader struct {
	data []byte
}

func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.data) == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	n := copy(p, f.data)
	f.data = f.data[n:]
	return n, nil
}

func TestResumableUploadPublishesFile(t *testing.T) {
	e := newEnv(t, 0)
	ctx := context.Background()
	payload := bytes.Repeat([]byte("abcdefgh"), 1000)

	u, err := e.svc.Create(ctx, 1, CreateInput{Size: int64(len(payload)), Dir: files.Root, Name: "video.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	u, err = e.svc.Write(ctx, 1, u.ID, 0, &failingReader{data: payload[:3000]})
	if !errors.Is(err, io.ErrUnexpectedEOF) || u.Offset != 3000 {
		t.Fatalf("interrupted Write() = offset %d, %v", u.Offset, err)
	}
	if _, err := e.svc.Write(ctx, 1, u.ID, 0, bytes.NewReader(payload)); !errors.Is(err, ErrOffsetMismatch) {
		t.Fatalf("stale offset error = %v", err)
	}
	u, err = e.svc.Write(ctx, 1, u.ID, 3000, bytes.NewReader(payload[3000:]))
	if err != nil || !u.Done() || u.Result.String() != "video.mp4" {
		t.Fatalf("final Write() = %+v, %v", u, err)
	}
	got, err := os.ReadFile(filepath.Join(e.dir, "users", "1", "video.mp4"))
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("published content mismatch: %v", err)
	}
	if _, err := e.svc.Get(ctx, 1, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("finished upload still tracked: %v", err)
	}
}

func TestWriteRejectsBodyLongerThanDeclared(t *testing.T) {
	e := newEnv(t, 0)
	ctx := context.Background()
	u, err := e.svc.Create(ctx, 1, CreateInput{Size: 4, Dir: files.Root, Name: "a.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Write(ctx, 1, u.ID, 0, strings.NewReader("abcdef")); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("oversized body error = %v", err)
	}
}

func TestCreateEnforcesQuotaIncludingReservations(t *testing.T) {
	e := newEnv(t, 100)
	ctx := context.Background()
	if _, err := e.svc.Create(ctx, 1, CreateInput{Size: 60, Dir: files.Root, Name: "a.bin"}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Create(ctx, 1, CreateInput{Size: 60, Dir: files.Root, Name: "b.bin"}); !errors.Is(err, files.ErrQuotaExceeded) {
		t.Fatalf("second upload error = %v", err)
	}
}

func TestCreateValidatesInput(t *testing.T) {
	e := newEnv(t, 0)
	ctx := context.Background()
	missing, _ := files.ParsePath("missing")
	cases := []struct {
		name string
		in   CreateInput
		want error
	}{
		{name: "negative size", in: CreateInput{Size: -1, Name: "a"}, want: ErrInvalidSize},
		{name: "bad name", in: CreateInput{Size: 1, Name: "../a"}, want: files.ErrInvalidName},
		{name: "missing folder", in: CreateInput{Size: 1, Dir: missing, Name: "a"}, want: files.ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := e.svc.Create(ctx, 1, tc.in); !errors.Is(err, tc.want) {
				t.Fatalf("Create() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestCreateLimitsUnfinishedUploads(t *testing.T) {
	e := newEnv(t, 0)
	ctx := context.Background()
	for i := range MaxActive {
		e.store.uploads[strconv.Itoa(i)] = Upload{ID: strconv.Itoa(i), UserID: 1, Size: 1}
	}
	if _, err := e.svc.Create(ctx, 1, CreateInput{Size: 1, Dir: files.Root, Name: "a.bin"}); !errors.Is(err, ErrTooManyUploads) {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestEmptyUploadIsPublishedImmediately(t *testing.T) {
	e := newEnv(t, 0)
	u, err := e.svc.Create(context.Background(), 1, CreateInput{Size: 0, Dir: files.Root, Name: "empty.txt"})
	if err != nil || u.Result.String() != "empty.txt" {
		t.Fatalf("Create() = %+v, %v", u, err)
	}
}

func TestPurgeExpiredRemovesStaging(t *testing.T) {
	e := newEnv(t, 0)
	ctx := context.Background()
	u, err := e.svc.Create(ctx, 1, CreateInput{Size: 10, Dir: files.Root, Name: "a.bin"})
	if err != nil {
		t.Fatal(err)
	}
	e.now = e.now.Add(25 * time.Hour)
	if n, err := e.svc.PurgeExpired(ctx); err != nil || n != 1 {
		t.Fatalf("PurgeExpired() = %d, %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(e.dir, ".system", "uploads", "1", u.ID+".part")); !os.IsNotExist(err) {
		t.Fatalf("staging file left behind: %v", err)
	}
}
