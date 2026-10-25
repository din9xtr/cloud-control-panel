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

func (m *memoryStore) ReadyUpload(_ context.Context, id string, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.uploads[id]
	u.Offset, u.Ready, u.ExpiresAt = u.Size, true, expiresAt
	m.uploads[id] = u
	return nil
}

func (m *memoryStore) CompleteUpload(_ context.Context, id string, result files.Path) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.uploads[id]
	u.Result = result
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
		if u.UserID == userID && !u.Done() {
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
		if u.UserID == userID && !u.Done() {
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
	if receipt, err := e.svc.Get(ctx, 1, u.ID); err != nil || !receipt.Done() {
		t.Fatalf("missing completion receipt: %+v, %v", receipt, err)
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

func TestAcknowledgedPrefixCannotBeRecreatedWithZeroes(t *testing.T) {
	e := newEnv(t, 0)
	ctx := context.Background()
	u, err := e.svc.Create(ctx, 1, CreateInput{Size: 6, Name: "notes.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Write(ctx, 1, u.ID, 0, strings.NewReader("abc")); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(filepath.Join(e.dir, ".system", "uploads", "1", u.ID+".part"), 1); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Write(ctx, 1, u.ID, 3, strings.NewReader("def")); err == nil {
		t.Fatal("published a file with a missing acknowledged prefix")
	}
	if _, err := os.Stat(filepath.Join(e.dir, "users", "1", "notes.txt")); !os.IsNotExist(err) {
		t.Fatalf("corrupt file published: %v", err)
	}
}

func TestRejectedFinalBodyCanBeRetried(t *testing.T) {
	for _, body := range []io.Reader{strings.NewReader("abcdextra"), &failingReader{data: []byte("abcd")}} {
		e := newEnv(t, 0)
		ctx := context.Background()
		u, err := e.svc.Create(ctx, 1, CreateInput{Size: 4, Name: "notes.txt"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.svc.Write(ctx, 1, u.ID, 0, body); err == nil {
			t.Fatal("accepted unvalidated final body")
		}
		u, err = e.svc.Get(ctx, 1, u.ID)
		if err != nil || u.Offset != 0 {
			t.Fatalf("rejected final body advanced offset: %+v, %v", u, err)
		}
		if _, err := e.svc.Write(ctx, 1, u.ID, 0, strings.NewReader("abcd")); err != nil {
			t.Fatal(err)
		}
	}
}

type completionFailure struct {
	Store
	fail bool
}

func (s *completionFailure) CompleteUpload(ctx context.Context, id string, result files.Path) error {
	if s.fail {
		return errors.New("injected completion failure")
	}
	return s.Store.CompleteUpload(ctx, id, result)
}

func restartService(s *Service) *Service {
	return NewService(Config{Store: s.store, Staging: s.staging, Library: s.library, Clock: s.clock, TTL: s.ttl})
}

func TestPublicationRecoversAfterCompletionFailureAndRestart(t *testing.T) {
	e := newEnv(t, 100)
	ctx := context.Background()
	store := &completionFailure{Store: e.store, fail: true}
	e.svc.store = store
	u, err := e.svc.Create(ctx, 1, CreateInput{Size: 4, Name: "notes.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Write(ctx, 1, u.ID, 0, strings.NewReader("abcd")); err == nil {
		t.Fatal("expected completion failure")
	}
	store.fail = false
	e.svc = restartService(e.svc)
	receipt, err := e.svc.Status(ctx, 1, u.ID)
	if err != nil || !receipt.Done() || receipt.Offset != 4 {
		t.Fatalf("recover: %+v, %v", receipt, err)
	}
	entries, err := os.ReadDir(filepath.Join(e.dir, "users", "1"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "notes.txt" {
		t.Fatalf("duplicate publication: %v, %v", entries, err)
	}
	if n, err := e.svc.Reserved(ctx, 1); err != nil || n != 0 {
		t.Fatalf("completed reservation = %d, %v", n, err)
	}
	if err := e.svc.Terminate(ctx, 1, u.ID); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(e.dir, "users", "1", "notes.txt"))
	if err != nil || string(data) != "abcd" {
		t.Fatalf("receipt deletion changed file: %q, %v", data, err)
	}
}

func TestConcurrentQuotaReservations(t *testing.T) {
	e := newEnv(t, 100)
	var wg sync.WaitGroup
	results := make(chan error, 20)
	for range 20 {
		wg.Go(func() {
			_, err := e.svc.Create(context.Background(), 1, CreateInput{Size: 30, Name: "notes.txt"})
			results <- err
		})
	}
	wg.Wait()
	close(results)
	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
		} else if !errors.Is(err, files.ErrQuotaExceeded) {
			t.Fatal(err)
		}
	}
	if accepted != 3 {
		t.Fatalf("accepted %d uploads beyond quota", accepted)
	}
}

type staleExpiry struct {
	Store
	extend func()
}

func (s staleExpiry) ExpiredUploads(ctx context.Context, now time.Time) ([]Upload, error) {
	result, err := s.Store.ExpiredUploads(ctx, now)
	s.extend()
	return result, err
}

func TestPurgeRechecksRefreshedExpiry(t *testing.T) {
	e := newEnv(t, 0)
	ctx := context.Background()
	u, err := e.svc.Create(ctx, 1, CreateInput{Size: 4, Name: "notes.txt"})
	if err != nil {
		t.Fatal(err)
	}
	e.now = e.now.Add(25 * time.Hour)
	e.svc.store = staleExpiry{Store: e.store, extend: func() { _ = e.store.UpdateOffset(ctx, u.ID, 0, e.now.Add(time.Hour)) }}
	if n, err := e.svc.PurgeExpired(ctx); n != 0 || err != nil {
		t.Fatalf("purged refreshed upload: %d, %v", n, err)
	}
	if _, err := e.svc.Get(ctx, 1, u.ID); err != nil {
		t.Fatal(err)
	}
}

type readonlyStaging struct {
	Staging
	dir string
}

func (s readonlyStaging) OpenStaging(userID int64, id string) (*os.File, error) {
	return os.Open(filepath.Join(s.dir, ".system", "uploads", strconv.FormatInt(userID, 10), id+".part"))
}

func TestFailedStagingWriteKeepsAcknowledgedOffset(t *testing.T) {
	e := newEnv(t, 0)
	ctx := context.Background()
	u, err := e.svc.Create(ctx, 1, CreateInput{Size: 4, Name: "notes.txt"})
	if err != nil {
		t.Fatal(err)
	}
	staging := e.svc.staging
	e.svc.staging = readonlyStaging{Staging: staging, dir: e.dir}
	if _, err := e.svc.Write(ctx, 1, u.ID, 0, strings.NewReader("abcd")); err == nil {
		t.Fatal("expected staging failure")
	}
	u, err = e.svc.Get(ctx, 1, u.ID)
	if err != nil || u.Offset != 0 {
		t.Fatalf("acknowledged failed write: %+v, %v", u, err)
	}
	e.svc.staging = staging
	if _, err := e.svc.Write(ctx, 1, u.ID, 0, strings.NewReader("abcd")); err != nil {
		t.Fatal(err)
	}
}
