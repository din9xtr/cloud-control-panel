package auth

import (
	"sync"
	"time"
)

type LimiterPolicy struct {
	MaxFailures int
	Window      time.Duration
	Lockout     time.Duration
	MaxKeys     int
}

type attempts struct {
	failures    int
	firstAt     time.Time
	lockedUntil time.Time
}

type Limiter struct {
	policy LimiterPolicy
	clock  func() time.Time
	mu     sync.Mutex
	keys   map[string]*attempts
}

func NewLimiter(policy LimiterPolicy, clock func() time.Time) *Limiter {
	return &Limiter{policy: policy, clock: clock, keys: make(map[string]*attempts)}
}

func (l *Limiter) RetryAfter(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.keys[key]
	if !ok {
		return 0
	}
	if wait := a.lockedUntil.Sub(l.clock()); wait > 0 {
		return wait
	}
	return 0
}

func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.clock()
	a, ok := l.keys[key]
	if !ok || now.Sub(a.firstAt) > l.policy.Window {
		if !ok && len(l.keys) >= l.policy.MaxKeys && !l.evict(now) {
			return
		}
		a = &attempts{firstAt: now}
		l.keys[key] = a
	}
	a.failures++
	if a.failures >= l.policy.MaxFailures {
		a.lockedUntil = now.Add(l.policy.Lockout)
		a.failures = 0
		a.firstAt = now
	}
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.keys, key)
}

func (l *Limiter) evict(now time.Time) bool {
	for key, a := range l.keys {
		if now.Sub(a.firstAt) > l.policy.Window && now.After(a.lockedUntil) {
			delete(l.keys, key)
		}
	}
	for key, a := range l.keys {
		if len(l.keys) < l.policy.MaxKeys {
			break
		}
		if now.After(a.lockedUntil) {
			delete(l.keys, key)
		}
	}
	return len(l.keys) < l.policy.MaxKeys
}
