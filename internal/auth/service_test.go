package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type memoryStore struct {
	mu       sync.Mutex
	users    map[int64]User
	sessions map[string]Session
	nextID   int64
}

func newMemoryStore() *memoryStore {
	return &memoryStore{users: map[int64]User{}, sessions: map[string]Session{}}
}

func (m *memoryStore) CountUsers(context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.users), nil
}

func (m *memoryStore) CreateUser(_ context.Context, u User) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	u.ID = m.nextID
	m.users[u.ID] = u
	return u, nil
}

func (m *memoryStore) UserByID(_ context.Context, id int64) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return u, nil
}

func (m *memoryStore) UserByUsername(_ context.Context, username string) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Username == username {
			return u, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (m *memoryStore) UpdateCredentials(_ context.Context, u User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.ID] = u
	return nil
}

func (m *memoryStore) CreateSession(_ context.Context, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s
	return nil
}

func (m *memoryStore) SessionByID(_ context.Context, id string) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	return s, nil
}

func (m *memoryStore) TouchSession(_ context.Context, id string, at time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.sessions[id]
	s.LastSeenAt = at
	m.sessions[id] = s
	return nil
}

func (m *memoryStore) DeleteSession(_ context.Context, userID int64, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok || s.UserID != userID {
		return ErrSessionNotFound
	}
	delete(m.sessions, id)
	return nil
}

func (m *memoryStore) DeleteOtherSessions(_ context.Context, userID int64, keepID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		if s.UserID == userID && id != keepID {
			delete(m.sessions, id)
		}
	}
	return nil
}

func (m *memoryStore) UserSessions(_ context.Context, userID int64) ([]Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Session
	for _, s := range m.sessions {
		if s.UserID == userID {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *memoryStore) DeleteExpiredSessions(context.Context, time.Time, time.Time) (int64, error) {
	return 0, nil
}

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func newTestService(t *testing.T) (*Service, *memoryStore, *clock) {
	t.Helper()
	store := newMemoryStore()
	c := &clock{now: time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)}
	policy := LimiterPolicy{MaxFailures: 3, Window: 15 * time.Minute, Lockout: 15 * time.Minute, MaxKeys: 100}
	svc, err := NewService(Config{
		Users:       store,
		Sessions:    store,
		Hasher:      BcryptHasher{Cost: bcrypt.MinCost},
		IPLimiter:   NewLimiter(policy, c.Now),
		UserLimiter: NewLimiter(LimiterPolicy{MaxFailures: 10, Window: time.Hour, Lockout: time.Hour, MaxKeys: 100}, c.Now),
		Clock:       c.Now,
		SessionTTL:  30 * 24 * time.Hour,
		IdleTTL:     7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Setup(context.Background(), SetupInput{Username: "admin", Password: "correct-horse"}); err != nil {
		t.Fatal(err)
	}
	return svc, store, c
}

func login(t *testing.T, svc *Service, password string) (LoginResult, error) {
	t.Helper()
	return svc.Login(context.Background(), LoginInput{Username: "admin", Password: password, IP: "203.0.113.1", Network: "203.0.113.1", UserAgent: "test"})
}

func TestLoginAndAuthenticate(t *testing.T) {
	svc, _, c := newTestService(t)
	ctx := context.Background()

	res, err := login(t, svc, "correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Token) < 40 || strings.Contains(res.Principal.SessionID, res.Token) {
		t.Fatalf("unexpected token/session id: %q %q", res.Token, res.Principal.SessionID)
	}
	p, err := svc.Authenticate(ctx, res.Token)
	if err != nil || p.Username != "admin" {
		t.Fatalf("Authenticate() = %+v, %v", p, err)
	}
	if _, err := svc.Authenticate(ctx, res.Token+"x"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("tampered token error = %v", err)
	}
	c.now = c.now.Add(8 * 24 * time.Hour)
	if _, err := svc.Authenticate(ctx, res.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("idle session error = %v", err)
	}
}

func TestLoginThrottlesRepeatedFailures(t *testing.T) {
	svc, _, c := newTestService(t)
	for range 3 {
		if _, err := login(t, svc, "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("wrong password error = %v", err)
		}
	}
	_, err := login(t, svc, "correct-horse")
	var throttled *ThrottledError
	if !errors.As(err, &throttled) || throttled.RetryAfter <= 0 {
		t.Fatalf("expected throttling, got %v", err)
	}
	c.now = c.now.Add(16 * time.Minute)
	if _, err := login(t, svc, "correct-horse"); err != nil {
		t.Fatalf("login after lockout: %v", err)
	}
}

func TestUnknownUserLooksLikeWrongPassword(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.Login(context.Background(), LoginInput{Username: "ghost", Password: "whatever-pass", IP: "198.51.100.2", Network: "198.51.100.2"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("unknown user error = %v", err)
	}
}

func TestChangeCredentialsRevokesOtherSessions(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	current, _ := login(t, svc, "correct-horse")
	other, _ := login(t, svc, "correct-horse")

	_, err := svc.ChangeCredentials(ctx, current.Principal, ChangeCredentialsInput{CurrentPassword: "nope", NewPassword: "new-password-1"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong current password error = %v", err)
	}
	if _, err := svc.ChangeCredentials(ctx, current.Principal, ChangeCredentialsInput{CurrentPassword: "correct-horse", Username: "x"}); !errors.Is(err, ErrInvalidUsername) {
		t.Fatalf("invalid username error = %v", err)
	}
	user, err := svc.ChangeCredentials(ctx, current.Principal, ChangeCredentialsInput{
		CurrentPassword: "correct-horse",
		Username:        "owner",
		NewPassword:     "new-password-1",
	})
	if err != nil || user.Username != "owner" {
		t.Fatalf("ChangeCredentials() = %+v, %v", user, err)
	}
	if _, err := svc.Authenticate(ctx, other.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("other session still valid: %v", err)
	}
	if _, err := svc.Authenticate(ctx, current.Token); err != nil {
		t.Fatalf("current session revoked: %v", err)
	}
	if len(store.sessions) != 1 {
		t.Fatalf("sessions left = %d", len(store.sessions))
	}
}

func TestChangeCredentialsThrottlesPasswordGuessing(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	current, err := login(t, svc, "correct-horse")
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		_, err := svc.ChangeCredentials(ctx, current.Principal, ChangeCredentialsInput{CurrentPassword: "guess-guess", NewPassword: "new-password-1"})
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("wrong current password error = %v", err)
		}
	}
	_, err = svc.ChangeCredentials(ctx, current.Principal, ChangeCredentialsInput{CurrentPassword: "correct-horse", NewPassword: "new-password-1"})
	var throttled *ThrottledError
	if !errors.As(err, &throttled) {
		t.Fatalf("expected throttling, got %v", err)
	}
}

func TestSetupCreatesTheFirstAccountOnlyOnce(t *testing.T) {
	store := newMemoryStore()
	c := &clock{now: time.Now()}
	policy := LimiterPolicy{MaxFailures: 5, Window: time.Minute, Lockout: time.Minute, MaxKeys: 10}
	svc, err := NewService(Config{
		Users: store, Sessions: store, Hasher: BcryptHasher{Cost: bcrypt.MinCost},
		IPLimiter: NewLimiter(policy, c.Now), UserLimiter: NewLimiter(policy, c.Now),
		Clock: c.Now, SessionTTL: time.Hour, IdleTTL: time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if required, _ := svc.SetupRequired(ctx); !required {
		t.Fatal("setup must be required on an empty store")
	}
	if _, err := svc.Setup(ctx, SetupInput{Username: "owner", Password: "short"}); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("weak password error = %v", err)
	}
	res, err := svc.Setup(ctx, SetupInput{Username: "owner", Password: "long-enough-pass"})
	if err != nil || res.Token == "" || res.Principal.Username != "owner" {
		t.Fatalf("Setup() = %+v, %v", res, err)
	}
	if _, err := svc.Setup(ctx, SetupInput{Username: "intruder", Password: "long-enough-pass"}); !errors.Is(err, ErrAlreadySetUp) {
		t.Fatalf("second Setup() error = %v", err)
	}
	if required, _ := svc.SetupRequired(ctx); required {
		t.Fatal("setup must not be required after the first account")
	}
}

func TestBcryptAcceptsLegacyPHPHashes(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("legacy-secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	phpHash := "$2y$" + string(hash[4:])
	if err := (BcryptHasher{}).Compare(phpHash, "legacy-secret"); err != nil {
		t.Fatalf("legacy $2y$ hash rejected: %v", err)
	}
}

func TestValidateUsername(t *testing.T) {
	for _, ok := range []string{"admin", "din.9-x_t"} {
		if err := ValidateUsername(ok); err != nil {
			t.Errorf("ValidateUsername(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"ab", "with space", "юзер", strings.Repeat("a", 33), "a/b"} {
		if err := ValidateUsername(bad); err == nil {
			t.Errorf("ValidateUsername(%q) accepted", bad)
		}
	}
}
