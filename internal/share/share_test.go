package share

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/din9xtr/cloud-control-panel/internal/files"
)

type memoryStore struct {
	links map[string]Link
}

func (m *memoryStore) CreateLink(_ context.Context, l Link) error {
	m.links[l.Token] = l
	return nil
}

func (m *memoryStore) LinkByToken(_ context.Context, token string) (Link, error) {
	l, ok := m.links[token]
	if !ok {
		return Link{}, ErrLinkNotFound
	}
	return l, nil
}

func (m *memoryStore) ActiveLinks(context.Context, int64, time.Time) ([]Link, error)  { return nil, nil }
func (m *memoryStore) DeleteLink(context.Context, int64, string) error                { return nil }
func (m *memoryStore) IncrementDownloads(context.Context, string) error               { return nil }
func (m *memoryStore) MoveLinks(context.Context, int64, files.Path, files.Path) error { return nil }
func (m *memoryStore) DeleteLinksUnder(context.Context, int64, files.Path) error      { return nil }
func (m *memoryStore) DeleteExpiredLinks(context.Context, time.Time) (int64, error)   { return 0, nil }

type stubFiles struct {
	kind files.Kind
	path string
}

func (s stubFiles) Stat(_ context.Context, _ int64, p files.Path) (files.Entry, error) {
	return files.Entry{Path: p, Kind: s.kind}, nil
}

func (s stubFiles) Open(_ context.Context, _ int64, p files.Path) (*os.File, files.Entry, error) {
	f, err := os.Open(s.path)
	return f, files.Entry{Path: p, Kind: s.kind}, err
}

type ttl time.Duration

func (t ttl) DefaultLinkTTL(context.Context) (time.Duration, error) { return time.Duration(t), nil }

func TestCreateUsesDefaultTTLAndExpires(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/f", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	svc := NewService(&memoryStore{links: map[string]Link{}}, stubFiles{kind: files.KindFile, path: dir + "/f"}, ttl(24*time.Hour), func() time.Time { return now })
	ctx := context.Background()
	p, _ := files.ParsePath("a/f")

	link, err := svc.Create(ctx, 1, p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if link.ExpiresAt.Sub(link.CreatedAt) != 24*time.Hour || len(link.Token) != 22 {
		t.Fatalf("link = %+v", link)
	}
	f, _, _, err := svc.Open(ctx, link.Token)
	if err != nil {
		t.Fatalf("Open() before expiry: %v", err)
	}
	f.Close()
	now = now.Add(24 * time.Hour)
	if _, _, _, err := svc.Open(ctx, link.Token); !errors.Is(err, ErrLinkNotFound) {
		t.Fatalf("Open() after expiry error = %v", err)
	}
}

func TestCreateAcceptsFoldersAndRejectsBadTTL(t *testing.T) {
	svc := NewService(&memoryStore{links: map[string]Link{}}, stubFiles{kind: files.KindDir}, ttl(time.Hour), time.Now)
	p, _ := files.ParsePath("folder")
	if _, err := svc.Create(context.Background(), 1, p, 0); err != nil {
		t.Fatalf("folder error = %v", err)
	}
	if _, err := svc.Create(context.Background(), 1, p, time.Minute); !errors.Is(err, ErrInvalidTTL) {
		t.Fatalf("ttl error = %v", err)
	}
}

func TestFolderModesAndRelativePaths(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	svc := NewService(&memoryStore{links: map[string]Link{}}, stubFiles{kind: files.KindDir}, ttl(time.Hour), func() time.Time { return now })
	ctx := context.Background()
	p, _ := files.ParsePath("garden/greenhouse")
	link, err := svc.CreateWithMode(ctx, 1, p, 0, ModeDownload)
	if err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{"", "seeds.txt", "shelves/seeds.txt"} {
		_, got, err := svc.ResolvePath(ctx, link.Token, relative)
		if err != nil || got.String() != relative {
			t.Fatalf("relative %q = %v, %v", relative, got, err)
		}
	}
	for _, relative := range []string{"..", "shelves/../../private", "shelves\\seeds.txt"} {
		if _, _, err := svc.ResolvePath(ctx, link.Token, relative); err == nil {
			t.Fatalf("accepted %q", relative)
		}
	}
	drop, err := svc.CreateWithMode(ctx, 1, p, 0, ModeUpload)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.ResolvePath(ctx, drop.Token, ""); !errors.Is(err, ErrLinkNotFound) {
		t.Fatalf("upload link browsing = %v", err)
	}
	if _, _, _, err := svc.Open(ctx, drop.Token); !errors.Is(err, ErrLinkNotFound) {
		t.Fatalf("upload link download = %v", err)
	}
	now = now.Add(time.Hour)
	for _, token := range []string{link.Token, drop.Token} {
		if _, err := svc.Resolve(ctx, token); !errors.Is(err, ErrLinkNotFound) {
			t.Fatalf("expired link = %v", err)
		}
	}
}

func TestInvalidLinkModes(t *testing.T) {
	for _, tt := range []struct {
		name string
		kind files.Kind
		mode Mode
		path string
		want error
	}{
		{name: "upload requires folder", kind: files.KindFile, mode: ModeUpload, path: "seeds.txt", want: ErrInvalidMode},
		{name: "unknown mode", kind: files.KindDir, mode: "edit", path: "garden", want: ErrInvalidMode},
		{name: "root folder", kind: files.KindDir, mode: ModeDownload, want: files.ErrRootOperation},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(&memoryStore{links: map[string]Link{}}, stubFiles{kind: tt.kind}, ttl(time.Hour), time.Now)
			p, _ := files.ParsePath(tt.path)
			if _, err := svc.CreateWithMode(context.Background(), 1, p, 0, tt.mode); !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}
