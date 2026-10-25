package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/din9xtr/cloud-control-panel/internal/auth"
)

type Users struct {
	db *sql.DB
}

var _ auth.UserStore = (*Users)(nil)

func NewUsers(db *sql.DB) *Users {
	return &Users{db: db}
}

const userColumns = `id, username, password, created_at, password_changed_at`

func (s *Users) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Users) CreateUser(ctx context.Context, u auth.User) (auth.User, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password, created_at, password_changed_at) VALUES (?, ?, ?, ?)`,
		u.Username, u.PasswordHash, unix(u.CreatedAt), unix(u.PasswordChangedAt))
	if isUniqueViolation(err) {
		return auth.User{}, auth.ErrUsernameTaken
	}
	if err != nil {
		return auth.User{}, err
	}
	if u.ID, err = res.LastInsertId(); err != nil {
		return auth.User{}, err
	}
	return u, nil
}

func (s *Users) UserByID(ctx context.Context, id int64) (auth.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

func (s *Users) UserByUsername(ctx context.Context, username string) (auth.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE username = ?`, username))
}

func (s *Users) UpdateCredentials(ctx context.Context, u auth.User) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET username = ?, password = ?, password_changed_at = ? WHERE id = ?`,
		u.Username, u.PasswordHash, unix(u.PasswordChangedAt), u.ID)
	if isUniqueViolation(err) {
		return auth.ErrUsernameTaken
	}
	if err != nil {
		return err
	}
	return expectOne(res, auth.ErrUserNotFound)
}

func scanUser(row *sql.Row) (auth.User, error) {
	var (
		u         auth.User
		createdAt int64
		changedAt sql.NullInt64
	)
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &createdAt, &changedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.User{}, auth.ErrUserNotFound
	}
	if err != nil {
		return auth.User{}, err
	}
	u.CreatedAt = fromUnix(createdAt)
	u.PasswordChangedAt = nullableUnix(changedAt)
	if u.PasswordChangedAt.IsZero() {
		u.PasswordChangedAt = u.CreatedAt
	}
	return u, nil
}

type Sessions struct {
	db *sql.DB
}

var _ auth.SessionStore = (*Sessions)(nil)

func NewSessions(db *sql.DB) *Sessions {
	return &Sessions{db: db}
}

const sessionColumns = `id, user_id, created_at, last_seen_at, expires_at, ip, user_agent`

func (s *Sessions) CreateSession(ctx context.Context, sess auth.Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO auth_sessions (`+sessionColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sess.ID, sess.UserID, unix(sess.CreatedAt), unix(sess.LastSeenAt), unix(sess.ExpiresAt), sess.IP, sess.UserAgent)
	return err
}

func (s *Sessions) SessionByID(ctx context.Context, id string) (auth.Session, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+sessionColumns+` FROM auth_sessions WHERE id = ?`, id)
	sess, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.Session{}, auth.ErrSessionNotFound
	}
	return sess, err
}

func (s *Sessions) TouchSession(ctx context.Context, id string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE auth_sessions SET last_seen_at = ? WHERE id = ?`, unix(at), id)
	return err
}

func (s *Sessions) DeleteSession(ctx context.Context, userID int64, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	return expectOne(res, auth.ErrSessionNotFound)
}

func (s *Sessions) DeleteOtherSessions(ctx context.Context, userID int64, keepID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id = ? AND id <> ?`, userID, keepID)
	return err
}

func (s *Sessions) UserSessions(ctx context.Context, userID int64) ([]auth.Session, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+sessionColumns+` FROM auth_sessions WHERE user_id = ? ORDER BY last_seen_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []auth.Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

func (s *Sessions) DeleteExpiredSessions(ctx context.Context, now, idleBefore time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM auth_sessions WHERE expires_at <= ? OR last_seen_at < ?`, unix(now), unix(idleBefore))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanSession(row scanner) (auth.Session, error) {
	var (
		sess                         auth.Session
		createdAt, seenAt, expiresAt int64
	)
	if err := row.Scan(&sess.ID, &sess.UserID, &createdAt, &seenAt, &expiresAt, &sess.IP, &sess.UserAgent); err != nil {
		return auth.Session{}, err
	}
	sess.CreatedAt, sess.LastSeenAt, sess.ExpiresAt = fromUnix(createdAt), fromUnix(seenAt), fromUnix(expiresAt)
	return sess, nil
}
