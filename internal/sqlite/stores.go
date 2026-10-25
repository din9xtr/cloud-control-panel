package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/din9xtr/cloud-control-panel/internal/files"
	"github.com/din9xtr/cloud-control-panel/internal/settings"
	"github.com/din9xtr/cloud-control-panel/internal/share"
	"github.com/din9xtr/cloud-control-panel/internal/upload"
)

type Links struct {
	db *sql.DB
}

var _ share.Store = (*Links)(nil)

func NewLinks(db *sql.DB) *Links {
	return &Links{db: db}
}

const linkColumns = `id, user_id, path, token, created_at, expires_at, downloads, mode, kind`

func (s *Links) CreateLink(ctx context.Context, l share.Link) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO share_links (`+linkColumns+`) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		l.ID, l.UserID, l.Path.String(), l.Token, unix(l.CreatedAt), unix(l.ExpiresAt), defaultLinkMode(l.Mode), defaultLinkKind(l.Kind))
	return err
}

func (s *Links) LinkByToken(ctx context.Context, token string) (share.Link, error) {
	l, err := scanLink(s.db.QueryRowContext(ctx, `SELECT `+linkColumns+` FROM share_links WHERE token = ?`, token))
	if errors.Is(err, sql.ErrNoRows) {
		return share.Link{}, share.ErrLinkNotFound
	}
	return l, err
}

func (s *Links) ActiveLinks(ctx context.Context, userID int64, now time.Time) ([]share.Link, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+linkColumns+` FROM share_links WHERE user_id = ? AND expires_at > ? ORDER BY created_at DESC`, userID, unix(now))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []share.Link
	for rows.Next() {
		l, err := scanLink(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Links) DeleteLink(ctx context.Context, userID int64, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM share_links WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	return expectOne(res, share.ErrLinkNotFound)
}

func (s *Links) IncrementDownloads(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE share_links SET downloads = downloads + 1 WHERE id = ?`, id)
	return err
}

func (s *Links) MoveLinks(ctx context.Context, userID int64, from, to files.Path) error {
	fromLen := utf8.RuneCountInString(from.String())
	prefix := from.String() + "/"
	replacement := to.String()
	_, err := s.db.ExecContext(ctx, `
		UPDATE share_links
		SET path = CASE WHEN path = ? THEN ? ELSE ? || substr(path, ?) END
		WHERE user_id = ? AND (path = ? OR substr(path, 1, ?) = ?)`,
		from.String(), replacement, replacement, fromLen+1,
		userID, from.String(), fromLen+1, prefix)
	return err
}

func (s *Links) DeleteLinksUnder(ctx context.Context, userID int64, p files.Path) error {
	prefix := p.String() + "/"
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM share_links WHERE user_id = ? AND (path = ? OR substr(path, 1, ?) = ?)`,
		userID, p.String(), utf8.RuneCountInString(prefix), prefix)
	return err
}

func (s *Links) DeleteExpiredLinks(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM share_links WHERE expires_at <= ?`, unix(now))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func scanLink(row scanner) (share.Link, error) {
	var (
		l                    share.Link
		rawPath              string
		createdAt, expiresAt int64
	)
	if err := row.Scan(&l.ID, &l.UserID, &rawPath, &l.Token, &createdAt, &expiresAt, &l.Downloads, &l.Mode, &l.Kind); err != nil {
		return share.Link{}, err
	}
	p, err := files.ParsePath(rawPath)
	if err != nil {
		return share.Link{}, err
	}
	l.Path, l.CreatedAt, l.ExpiresAt = p, fromUnix(createdAt), fromUnix(expiresAt)
	return l, nil
}

type Trash struct {
	db *sql.DB
}

var _ files.TrashStore = (*Trash)(nil)

func NewTrash(db *sql.DB) *Trash {
	return &Trash{db: db}
}

const trashColumns = `id, user_id, original_path, kind, size, trashed_at`

func (s *Trash) AddTrashItem(ctx context.Context, item files.TrashItem) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO trash_items (`+trashColumns+`) VALUES (?, ?, ?, ?, ?, ?)`,
		item.ID, item.UserID, item.OriginalPath.String(), string(item.Kind), item.Size, unix(item.TrashedAt))
	return err
}

func (s *Trash) TrashItems(ctx context.Context, userID int64) ([]files.TrashItem, error) {
	return s.query(ctx, `SELECT `+trashColumns+` FROM trash_items WHERE user_id = ? ORDER BY trashed_at DESC`, userID)
}

func (s *Trash) TrashItem(ctx context.Context, userID int64, id string) (files.TrashItem, error) {
	item, err := scanTrash(s.db.QueryRowContext(ctx, `SELECT `+trashColumns+` FROM trash_items WHERE id = ? AND user_id = ?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return files.TrashItem{}, files.ErrTrashItemNotFound
	}
	return item, err
}

func (s *Trash) DeleteTrashItem(ctx context.Context, userID int64, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM trash_items WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	return expectOne(res, files.ErrTrashItemNotFound)
}

func (s *Trash) ExpiredTrashItems(ctx context.Context, before time.Time) ([]files.TrashItem, error) {
	return s.query(ctx, `SELECT `+trashColumns+` FROM trash_items WHERE trashed_at < ?`, unix(before))
}

func (s *Trash) query(ctx context.Context, q string, args ...any) ([]files.TrashItem, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []files.TrashItem
	for rows.Next() {
		item, err := scanTrash(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanTrash(row scanner) (files.TrashItem, error) {
	var (
		item      files.TrashItem
		rawPath   string
		kind      string
		trashedAt int64
	)
	if err := row.Scan(&item.ID, &item.UserID, &rawPath, &kind, &item.Size, &trashedAt); err != nil {
		return files.TrashItem{}, err
	}
	p, err := files.ParsePath(rawPath)
	if err != nil {
		return files.TrashItem{}, err
	}
	item.OriginalPath, item.Kind, item.TrashedAt = p, files.Kind(kind), fromUnix(trashedAt)
	return item, nil
}

type Uploads struct {
	db *sql.DB
}

var _ upload.Store = (*Uploads)(nil)

func NewUploads(db *sql.DB) *Uploads {
	return &Uploads{db: db}
}

const uploadColumns = `id, user_id, size, received, dir, name, created_at, expires_at, share_id, ready, result`

func (s *Uploads) CreateUpload(ctx context.Context, u upload.Upload) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO uploads (`+uploadColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.UserID, u.Size, u.Offset, u.Dir.String(), u.Name, unix(u.CreatedAt), unix(u.ExpiresAt), u.ShareID, u.Ready, u.Result.String())
	return err
}

func (s *Uploads) Upload(ctx context.Context, userID int64, id string) (upload.Upload, error) {
	u, err := scanUpload(s.db.QueryRowContext(ctx, `SELECT `+uploadColumns+` FROM uploads WHERE id = ? AND user_id = ?`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return upload.Upload{}, upload.ErrNotFound
	}
	return u, err
}

func (s *Uploads) UpdateOffset(ctx context.Context, id string, offset int64, expiresAt time.Time) error {
	res, err := s.db.ExecContext(ctx, `UPDATE uploads SET received = ?, expires_at = ? WHERE id = ?`, offset, unix(expiresAt), id)
	if err != nil {
		return err
	}
	return expectOne(res, upload.ErrNotFound)
}

func (s *Uploads) ReadyUpload(ctx context.Context, id string, expiresAt time.Time) error {
	res, err := s.db.ExecContext(ctx, `UPDATE uploads SET received = size, ready = 1, expires_at = ? WHERE id = ?`, unix(expiresAt), id)
	if err != nil {
		return err
	}
	return expectOne(res, upload.ErrNotFound)
}

func (s *Uploads) CompleteUpload(ctx context.Context, id string, result files.Path) error {
	res, err := s.db.ExecContext(ctx, `UPDATE uploads SET result = ? WHERE id = ? AND ready = 1`, result.String(), id)
	if err != nil {
		return err
	}
	return expectOne(res, upload.ErrNotFound)
}

func (s *Uploads) DeleteUpload(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM uploads WHERE id = ?`, id)
	if err != nil {
		return err
	}
	return expectOne(res, upload.ErrNotFound)
}

func (s *Uploads) ReservedBytes(ctx context.Context, userID int64) (int64, error) {
	var total int64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(size), 0) FROM uploads WHERE user_id = ? AND result = ''`, userID).Scan(&total)
	return total, err
}

func (s *Uploads) ActiveUploads(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM uploads WHERE user_id = ? AND result = ''`, userID).Scan(&n)
	return n, err
}

func (s *Uploads) ExpiredUploads(ctx context.Context, now time.Time) ([]upload.Upload, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+uploadColumns+` FROM uploads WHERE expires_at <= ?`, unix(now))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []upload.Upload
	for rows.Next() {
		u, err := scanUpload(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func scanUpload(row scanner) (upload.Upload, error) {
	var (
		u                    upload.Upload
		rawDir, rawResult    string
		createdAt, expiresAt int64
	)
	if err := row.Scan(&u.ID, &u.UserID, &u.Size, &u.Offset, &rawDir, &u.Name, &createdAt, &expiresAt, &u.ShareID, &u.Ready, &rawResult); err != nil {
		return upload.Upload{}, err
	}
	dir, err := files.ParsePath(rawDir)
	if err != nil {
		return upload.Upload{}, err
	}
	u.Result, err = files.ParsePath(rawResult)
	if err != nil {
		return upload.Upload{}, err
	}
	u.Dir, u.CreatedAt, u.ExpiresAt = dir, fromUnix(createdAt), fromUnix(expiresAt)
	return u, nil
}

type Settings struct {
	db *sql.DB
}

var _ settings.Store = (*Settings)(nil)

func NewSettings(db *sql.DB) *Settings {
	return &Settings{db: db}
}

func (s *Settings) LoadSettings(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		values[k] = v
	}
	return values, rows.Err()
}

func (s *Settings) SaveSettings(ctx context.Context, values map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for k, v := range values {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = excluded.value`, k, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func defaultLinkMode(mode share.Mode) share.Mode {
	if mode == "" {
		return share.ModeDownload
	}
	return mode
}

func defaultLinkKind(kind files.Kind) files.Kind {
	if kind == "" {
		return files.KindFile
	}
	return kind
}
