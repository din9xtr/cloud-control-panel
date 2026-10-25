package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/din9xtr/cloud-control-panel/internal/auth"
	"github.com/din9xtr/cloud-control-panel/internal/files"
	"github.com/din9xtr/cloud-control-panel/internal/settings"
	"github.com/din9xtr/cloud-control-panel/internal/share"
	"github.com/din9xtr/cloud-control-panel/internal/upload"
)

const legacySchema = `
CREATE TABLE migrations (id INTEGER PRIMARY KEY AUTOINCREMENT, migration TEXT NOT NULL UNIQUE, ran_at INTEGER NOT NULL);
CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE, password TEXT NOT NULL, created_at INTEGER NOT NULL);
CREATE TABLE sessions (id TEXT PRIMARY KEY, user_id INTEGER NOT NULL, auth_token TEXT NOT NULL UNIQUE, ip TEXT, user_agent TEXT,
  created_at INTEGER NOT NULL, last_activity_at INTEGER NOT NULL, revoked_at INTEGER, FOREIGN KEY (user_id) REFERENCES users(id));
CREATE TABLE login_throttle (id INTEGER PRIMARY KEY AUTOINCREMENT, ip TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 1, last_attempt INTEGER NOT NULL);
INSERT INTO users (username, password, created_at) VALUES ('legacy', '$2y$10$abcdefghijklmnopqrstuuJ8Xv1sGQm8u6n9cHq1P8aS3bJ9m7hO6', 1767225600);
INSERT INTO sessions VALUES ('s1', 1, 'tok', '127.0.0.1', 'ua', 1, 1, NULL);
`

func openTest(t *testing.T, seed string) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "database.sqlite")
	if seed != "" {
		raw, err := sql.Open("sqlite", path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := raw.Exec(seed); err != nil {
			t.Fatal(err)
		}
		raw.Close()
	}
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestMigratesLegacyDatabaseKeepingAdmin(t *testing.T) {
	db := openTest(t, legacySchema)
	ctx := context.Background()
	users := NewUsers(db)

	u, err := users.UserByUsername(ctx, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID != 1 || u.PasswordHash[:4] != "$2y$" || !u.PasswordChangedAt.Equal(u.CreatedAt) {
		t.Fatalf("legacy user = %+v", u)
	}
	var legacySessions int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&legacySessions); err != nil || legacySessions != 1 {
		t.Fatalf("legacy table touched: %d, %v", legacySessions, err)
	}
	fresh, err := Open(ctx, filepath.Join(t.TempDir(), "again.sqlite"))
	if err != nil {
		t.Fatalf("fresh database: %v", err)
	}
	if err := fresh.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestFolderLinksMigrationPreservesExistingLinksAndUploads(t *testing.T) {
	initial, err := migrationFiles.ReadFile("migrations/0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	defaults, err := migrationFiles.ReadFile("migrations/0002_default_settings.sql")
	if err != nil {
		t.Fatal(err)
	}
	seed := string(initial) + string(defaults) + `
CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL);
INSERT INTO schema_migrations VALUES (1, 1), (2, 1);
INSERT INTO users (id, username, password, created_at, password_changed_at) VALUES (1, 'gardener', 'hash', 1, 1);
INSERT INTO share_links (id, user_id, path, token, created_at, expires_at, downloads) VALUES ('link', 1, 'garden/seeds.txt', 'token', 1, 9999999999, 3);
INSERT INTO uploads (id, user_id, size, received, dir, name, created_at, expires_at) VALUES ('upload', 1, 10, 4, 'garden', 'catalog.txt', 1, 9999999999);
`
	db := openTest(t, seed)
	ctx := context.Background()
	link, err := NewLinks(db).LinkByToken(ctx, "token")
	if err != nil {
		t.Fatal(err)
	}
	if link.Mode != share.ModeDownload || link.Kind != files.KindFile || link.Downloads != 3 {
		t.Fatalf("migrated link = %+v", link)
	}
	u, err := NewUploads(db).Upload(ctx, 1, "upload")
	if err != nil {
		t.Fatal(err)
	}
	if u.ShareID != "" || u.Offset != 4 {
		t.Fatalf("migrated upload = %+v", u)
	}
}

func TestUsersAndSessions(t *testing.T) {
	db := openTest(t, "")
	ctx := context.Background()
	users, sessions := NewUsers(db), NewSessions(db)
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)

	u, err := users.CreateUser(ctx, auth.User{Username: "admin", PasswordHash: "h", CreatedAt: now, PasswordChangedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := users.CreateUser(ctx, auth.User{Username: "admin", PasswordHash: "h", CreatedAt: now}); !errors.Is(err, auth.ErrUsernameTaken) {
		t.Fatalf("duplicate user error = %v", err)
	}
	for _, id := range []string{"a", "b"} {
		err := sessions.CreateSession(ctx, auth.Session{ID: id, UserID: u.ID, CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := sessions.DeleteOtherSessions(ctx, u.ID, "a"); err != nil {
		t.Fatal(err)
	}
	list, err := sessions.UserSessions(ctx, u.ID)
	if err != nil || len(list) != 1 || list[0].ID != "a" {
		t.Fatalf("sessions = %+v, %v", list, err)
	}
	if err := sessions.DeleteSession(ctx, u.ID+1, "a"); !errors.Is(err, auth.ErrSessionNotFound) {
		t.Fatalf("foreign delete error = %v", err)
	}
	if n, err := sessions.DeleteExpiredSessions(ctx, now.Add(2*time.Hour), now); err != nil || n != 1 {
		t.Fatalf("DeleteExpiredSessions() = %d, %v", n, err)
	}
}

func TestLinksFollowRenamesWithUnicodePaths(t *testing.T) {
	db := openTest(t, "")
	ctx := context.Background()
	now := time.Now().UTC()
	u, _ := NewUsers(db).CreateUser(ctx, auth.User{Username: "admin", PasswordHash: "h", CreatedAt: now})
	links := NewLinks(db)
	mk := func(id, raw string) {
		p, _ := files.ParsePath(raw)
		if err := links.CreateLink(ctx, share.Link{ID: id, UserID: u.ID, Path: p, Token: "t" + id, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	mk("1", "фото/лето/a.jpg")
	mk("2", "фото/лето2/b.jpg")
	mk("3", "фото/лето")

	from, _ := files.ParsePath("фото/лето")
	to, _ := files.ParsePath("архив")
	if err := links.MoveLinks(ctx, u.ID, from, to); err != nil {
		t.Fatal(err)
	}
	active, err := links.ActiveLinks(ctx, u.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, l := range active {
		paths = append(paths, l.Path.String())
	}
	slices.Sort(paths)
	want := []string{"архив", "архив/a.jpg", "фото/лето2/b.jpg"}
	if !slices.Equal(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
	if err := links.DeleteLinksUnder(ctx, u.ID, to); err != nil {
		t.Fatal(err)
	}
	if active, _ = links.ActiveLinks(ctx, u.ID, now); len(active) != 1 {
		t.Fatalf("links after delete = %d", len(active))
	}
	if err := links.IncrementDownloads(ctx, "2"); err != nil {
		t.Fatal(err)
	}
	if l, _ := links.LinkByToken(ctx, "t2"); l.Downloads != 1 {
		t.Fatalf("downloads = %d", l.Downloads)
	}
}

func TestUploadsTrashAndSettings(t *testing.T) {
	db := openTest(t, "")
	ctx := context.Background()
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)
	u, _ := NewUsers(db).CreateUser(ctx, auth.User{Username: "admin", PasswordHash: "h", CreatedAt: now})

	uploads := NewUploads(db)
	dir, _ := files.ParsePath("docs")
	if err := uploads.CreateUpload(ctx, upload.Upload{ID: "u1", UserID: u.ID, Size: 100, Dir: dir, Name: "a.bin", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := uploads.UpdateOffset(ctx, "u1", 40, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := uploads.Upload(ctx, u.ID, "u1")
	if err != nil || got.Offset != 40 || got.Dir.String() != "docs" {
		t.Fatalf("upload = %+v, %v", got, err)
	}
	if reserved, _ := uploads.ReservedBytes(ctx, u.ID); reserved != 100 {
		t.Fatalf("reserved = %d", reserved)
	}
	if expired, _ := uploads.ExpiredUploads(ctx, now.Add(3*time.Hour)); len(expired) != 1 {
		t.Fatalf("expired = %v", expired)
	}

	trash := NewTrash(db)
	orig, _ := files.ParsePath("docs/old")
	if err := trash.AddTrashItem(ctx, files.TrashItem{ID: "t1", UserID: u.ID, OriginalPath: orig, Kind: files.KindDir, Size: 5, TrashedAt: now}); err != nil {
		t.Fatal(err)
	}
	if item, err := trash.TrashItem(ctx, u.ID, "t1"); err != nil || item.OriginalPath != orig {
		t.Fatalf("trash item = %+v, %v", item, err)
	}
	if err := trash.DeleteTrashItem(ctx, u.ID, "t1"); err != nil {
		t.Fatal(err)
	}
	if _, err := trash.TrashItem(ctx, u.ID, "t1"); !errors.Is(err, files.ErrTrashItemNotFound) {
		t.Fatalf("deleted item error = %v", err)
	}

	st := NewSettings(db)
	if err := st.SaveSettings(ctx, map[string]string{"a": "1"}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveSettings(ctx, map[string]string{"a": "2"}); err != nil {
		t.Fatal(err)
	}
	if values, _ := st.LoadSettings(ctx); values["a"] != "2" {
		t.Fatalf("settings = %v", values)
	}
}

func TestMigrationsSeedDefaultSettingsWithoutOverwriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cloud.db")
	ctx := context.Background()
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := settings.NewService(NewSettings(db)).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := settings.Settings{QuotaBytes: 0, TrashRetention: 30 * 24 * time.Hour, DefaultLinkTTL: 24 * time.Hour}
	if got != want {
		t.Fatalf("defaults = %+v, want %+v", got, want)
	}
	if err := NewSettings(db).SaveSettings(ctx, map[string]string{"quota_bytes": "1024"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = 2`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if values, _ := NewSettings(db).LoadSettings(ctx); values["quota_bytes"] != "1024" {
		t.Fatalf("saved quota overwritten: %v", values)
	}
}

func TestUploadReceiptSurvivesDatabaseReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "cloud.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	user, err := NewUsers(db).CreateUser(ctx, auth.User{Username: "gardener", PasswordHash: "h", CreatedAt: time.Now(), PasswordChangedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	store := NewUploads(db)
	now := time.Now().UTC().Truncate(time.Second)
	u := upload.Upload{ID: "receipt", UserID: user.ID, Size: 4, Name: "notes.txt", CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := store.CreateUpload(ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := store.ReadyUpload(ctx, u.ID, u.ExpiresAt); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store = NewUploads(db)
	ready, err := store.Upload(ctx, user.ID, u.ID)
	if err != nil || !ready.Ready || ready.Done() || ready.Offset != 4 {
		t.Fatalf("ready after reopen: %+v, %v", ready, err)
	}
	result, _ := files.ParsePath("notes.txt")
	if err := store.CompleteUpload(ctx, u.ID, result); err != nil {
		t.Fatal(err)
	}
	if reserved, err := store.ReservedBytes(ctx, user.ID); err != nil || reserved != 0 {
		t.Fatalf("reserved: %d, %v", reserved, err)
	}
	if active, err := store.ActiveUploads(ctx, user.ID); err != nil || active != 0 {
		t.Fatalf("active: %d, %v", active, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	receipt, err := NewUploads(db).Upload(ctx, user.ID, u.ID)
	if err != nil || !receipt.Done() || receipt.Result != result {
		t.Fatalf("receipt after reopen: %+v, %v", receipt, err)
	}
}
