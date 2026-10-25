package auth

import (
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

const (
	minUsernameLen = 3
	maxUsernameLen = 32
	minPasswordLen = 8
	maxPasswordLen = 72
	maxUserAgent   = 256
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrInvalidUsername    = errors.New("username must be 3-32 characters: letters, digits, dot, dash or underscore")
	ErrWeakPassword       = errors.New("password must be 8-72 bytes long")
	ErrUsernameTaken      = errors.New("username is taken")
	ErrUserNotFound       = errors.New("user not found")
	ErrSessionNotFound    = errors.New("session not found")
	ErrNothingToChange    = errors.New("nothing to change")
	ErrAlreadySetUp       = errors.New("the account is already set up")
	ErrBusy               = errors.New("server is busy, try again shortly")
)

type ThrottledError struct {
	RetryAfter time.Duration
}

func (e *ThrottledError) Error() string {
	return fmt.Sprintf("too many attempts, retry in %s", e.RetryAfter.Round(time.Second))
}

type User struct {
	ID                int64
	Username          string
	PasswordHash      string
	CreatedAt         time.Time
	PasswordChangedAt time.Time
}

type Session struct {
	ID         string
	UserID     int64
	CreatedAt  time.Time
	LastSeenAt time.Time
	ExpiresAt  time.Time
	IP         string
	UserAgent  string
}

type Principal struct {
	UserID    int64
	Username  string
	SessionID string
}

func ValidateUsername(name string) error {
	if len(name) < minUsernameLen || len(name) > maxUsernameLen {
		return ErrInvalidUsername
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
		default:
			return ErrInvalidUsername
		}
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < minPasswordLen || len(password) > maxPasswordLen || !utf8.ValidString(password) {
		return ErrWeakPassword
	}
	return nil
}

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	for limit > 0 && !utf8.RuneStart(s[limit]) {
		limit--
	}
	return s[:limit]
}
