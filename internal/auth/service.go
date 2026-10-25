package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	tokenBytes    = 32
	touchInterval = 5 * time.Minute
)

type UserStore interface {
	CountUsers(ctx context.Context) (int, error)
	CreateUser(ctx context.Context, u User) (User, error)
	UserByID(ctx context.Context, id int64) (User, error)
	UserByUsername(ctx context.Context, username string) (User, error)
	UpdateCredentials(ctx context.Context, u User) error
}

type SessionStore interface {
	CreateSession(ctx context.Context, s Session) error
	SessionByID(ctx context.Context, id string) (Session, error)
	TouchSession(ctx context.Context, id string, at time.Time) error
	DeleteSession(ctx context.Context, userID int64, id string) error
	DeleteOtherSessions(ctx context.Context, userID int64, keepID string) error
	UserSessions(ctx context.Context, userID int64) ([]Session, error)
	DeleteExpiredSessions(ctx context.Context, now, idleBefore time.Time) (int64, error)
}

type Config struct {
	Users       UserStore
	Sessions    SessionStore
	Hasher      Hasher
	IPLimiter   *Limiter
	UserLimiter *Limiter
	Clock       func() time.Time
	SessionTTL  time.Duration
	IdleTTL     time.Duration
}

type Service struct {
	users       UserStore
	sessions    SessionStore
	hasher      Hasher
	ipLimiter   *Limiter
	userLimiter *Limiter
	clock       func() time.Time
	sessionTTL  time.Duration
	idleTTL     time.Duration
	dummyHash   string
	setupMu     sync.Mutex
}

func NewService(cfg Config) (*Service, error) {
	dummy, err := cfg.Hasher.Hash("timing-equalizer-password")
	if err != nil {
		return nil, fmt.Errorf("prepare dummy hash: %w", err)
	}
	return &Service{
		users:       cfg.Users,
		sessions:    cfg.Sessions,
		hasher:      cfg.Hasher,
		ipLimiter:   cfg.IPLimiter,
		userLimiter: cfg.UserLimiter,
		clock:       cfg.Clock,
		sessionTTL:  cfg.SessionTTL,
		idleTTL:     cfg.IdleTTL,
		dummyHash:   dummy,
	}, nil
}

type LoginInput struct {
	Username  string
	Password  string
	IP        string
	Network   string
	UserAgent string
}

type LoginResult struct {
	Token     string
	Principal Principal
	ExpiresAt time.Time
}

func (s *Service) Login(ctx context.Context, in LoginInput) (LoginResult, error) {
	ipKey, userKey := "ip:"+in.Network, "user:"+in.Username
	if wait := max(s.ipLimiter.RetryAfter(ipKey), s.userLimiter.RetryAfter(userKey)); wait > 0 {
		return LoginResult{}, &ThrottledError{RetryAfter: wait}
	}
	user, err := s.verify(ctx, in.Username, in.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		s.ipLimiter.Fail(ipKey)
		s.userLimiter.Fail(userKey)
		return LoginResult{}, err
	}
	if err != nil {
		return LoginResult{}, err
	}
	s.ipLimiter.Reset(ipKey)
	s.userLimiter.Reset(userKey)
	return s.startSession(ctx, user, in.IP, in.UserAgent)
}

func (s *Service) startSession(ctx context.Context, user User, ip, userAgent string) (LoginResult, error) {
	token, session, err := s.newSession(user.ID, ip, userAgent)
	if err != nil {
		return LoginResult{}, err
	}
	if err := s.sessions.CreateSession(ctx, session); err != nil {
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}
	return LoginResult{
		Token:     token,
		Principal: Principal{UserID: user.ID, Username: user.Username, SessionID: session.ID},
		ExpiresAt: session.ExpiresAt,
	}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	if token == "" {
		return Principal{}, ErrUnauthenticated
	}
	id := sessionID(token)
	session, err := s.sessions.SessionByID(ctx, id)
	if errors.Is(err, ErrSessionNotFound) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, fmt.Errorf("load session: %w", err)
	}
	now := s.clock()
	if !now.Before(session.ExpiresAt) || now.Sub(session.LastSeenAt) > s.idleTTL {
		return Principal{}, ErrUnauthenticated
	}
	user, err := s.users.UserByID(ctx, session.UserID)
	if errors.Is(err, ErrUserNotFound) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, fmt.Errorf("load user: %w", err)
	}
	if now.Sub(session.LastSeenAt) > touchInterval {
		if err := s.sessions.TouchSession(ctx, id, now); err != nil {
			return Principal{}, fmt.Errorf("touch session: %w", err)
		}
	}
	return Principal{UserID: user.ID, Username: user.Username, SessionID: id}, nil
}

func (s *Service) Logout(ctx context.Context, p Principal) error {
	return s.sessions.DeleteSession(ctx, p.UserID, p.SessionID)
}

func (s *Service) User(ctx context.Context, id int64) (User, error) {
	return s.users.UserByID(ctx, id)
}

type ChangeCredentialsInput struct {
	CurrentPassword string
	Username        string
	NewPassword     string
}

func (s *Service) ChangeCredentials(ctx context.Context, p Principal, in ChangeCredentialsInput) (User, error) {
	user, err := s.users.UserByID(ctx, p.UserID)
	if err != nil {
		return User{}, err
	}
	userKey := "user:" + user.Username
	if wait := s.userLimiter.RetryAfter(userKey); wait > 0 {
		return User{}, &ThrottledError{RetryAfter: wait}
	}
	if err := s.hasher.Compare(user.PasswordHash, in.CurrentPassword); err != nil {
		if !errors.Is(err, ErrInvalidCredentials) {
			return User{}, fmt.Errorf("compare password: %w", err)
		}
		s.userLimiter.Fail(userKey)
		return User{}, ErrInvalidCredentials
	}
	usernameChanged := in.Username != "" && in.Username != user.Username
	passwordChanged := in.NewPassword != ""
	if !usernameChanged && !passwordChanged {
		return User{}, ErrNothingToChange
	}
	if usernameChanged {
		if err := ValidateUsername(in.Username); err != nil {
			return User{}, err
		}
		user.Username = in.Username
	}
	if passwordChanged {
		if err := ValidatePassword(in.NewPassword); err != nil {
			return User{}, err
		}
		if user.PasswordHash, err = s.hasher.Hash(in.NewPassword); err != nil {
			return User{}, fmt.Errorf("hash password: %w", err)
		}
		user.PasswordChangedAt = s.clock().UTC()
	}
	if err := s.users.UpdateCredentials(ctx, user); err != nil {
		return User{}, err
	}
	if passwordChanged {
		if err := s.sessions.DeleteOtherSessions(ctx, user.ID, p.SessionID); err != nil {
			return User{}, fmt.Errorf("revoke sessions: %w", err)
		}
	}
	return user, nil
}

func (s *Service) Sessions(ctx context.Context, userID int64) ([]Session, error) {
	return s.sessions.UserSessions(ctx, userID)
}

func (s *Service) RevokeSession(ctx context.Context, userID int64, id string) error {
	return s.sessions.DeleteSession(ctx, userID, id)
}

func (s *Service) PurgeExpiredSessions(ctx context.Context) (int64, error) {
	now := s.clock()
	return s.sessions.DeleteExpiredSessions(ctx, now, now.Add(-s.idleTTL))
}

func (s *Service) SetupRequired(ctx context.Context) (bool, error) {
	count, err := s.users.CountUsers(ctx)
	if err != nil {
		return false, fmt.Errorf("count users: %w", err)
	}
	return count == 0, nil
}

type SetupInput struct {
	Username  string
	Password  string
	IP        string
	UserAgent string
}

func (s *Service) Setup(ctx context.Context, in SetupInput) (LoginResult, error) {
	s.setupMu.Lock()
	defer s.setupMu.Unlock()
	required, err := s.SetupRequired(ctx)
	if err != nil {
		return LoginResult{}, err
	}
	if !required {
		return LoginResult{}, ErrAlreadySetUp
	}
	user, err := s.createUser(ctx, in.Username, in.Password)
	if err != nil {
		return LoginResult{}, err
	}
	return s.startSession(ctx, user, in.IP, in.UserAgent)
}

func (s *Service) SetPassword(ctx context.Context, username, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	user, err := s.users.UserByUsername(ctx, username)
	if err != nil {
		return err
	}
	if user.PasswordHash, err = s.hasher.Hash(password); err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	user.PasswordChangedAt = s.clock().UTC()
	if err := s.users.UpdateCredentials(ctx, user); err != nil {
		return err
	}
	return s.sessions.DeleteOtherSessions(ctx, user.ID, "")
}

func (s *Service) createUser(ctx context.Context, username, password string) (User, error) {
	if err := ValidateUsername(username); err != nil {
		return User{}, err
	}
	if err := ValidatePassword(password); err != nil {
		return User{}, err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}
	now := s.clock().UTC()
	return s.users.CreateUser(ctx, User{Username: username, PasswordHash: hash, CreatedAt: now, PasswordChangedAt: now})
}

func (s *Service) verify(ctx context.Context, username, password string) (User, error) {
	user, err := s.users.UserByUsername(ctx, username)
	if errors.Is(err, ErrUserNotFound) {
		_ = s.hasher.Compare(s.dummyHash, password)
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, fmt.Errorf("load user: %w", err)
	}
	if err := s.hasher.Compare(user.PasswordHash, password); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return User{}, err
		}
		return User{}, fmt.Errorf("compare password: %w", err)
	}
	return user, nil
}

func (s *Service) newSession(userID int64, ip, userAgent string) (string, Session, error) {
	token, err := randomToken(tokenBytes)
	if err != nil {
		return "", Session{}, err
	}
	now := s.clock().UTC()
	return token, Session{
		ID:         sessionID(token),
		UserID:     userID,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(s.sessionTTL),
		IP:         ip,
		UserAgent:  truncate(userAgent, maxUserAgent),
	}, nil
}

func sessionID(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
