package auth

import (
	"strconv"
	"testing"
	"time"
)

func TestLimiterKeepsLockoutsWhenFlooded(t *testing.T) {
	c := &clock{now: time.Now()}
	l := NewLimiter(LimiterPolicy{MaxFailures: 2, Window: time.Hour, Lockout: time.Hour, MaxKeys: 3}, c.Now)
	l.Fail("user:admin")
	l.Fail("user:admin")
	for i := range 100 {
		l.Fail("user:spray-" + strconv.Itoa(i))
	}
	if l.RetryAfter("user:admin") <= 0 {
		t.Fatal("lockout evicted by unrelated keys")
	}
	if len(l.keys) > 3 {
		t.Fatalf("limiter tracks %d keys, max 3", len(l.keys))
	}
}

func TestLimiterDropsExpiredKeysFirst(t *testing.T) {
	c := &clock{now: time.Now()}
	l := NewLimiter(LimiterPolicy{MaxFailures: 5, Window: time.Minute, Lockout: time.Minute, MaxKeys: 2}, c.Now)
	l.Fail("a")
	l.Fail("b")
	c.now = c.now.Add(2 * time.Minute)
	l.Fail("c")
	if _, ok := l.keys["c"]; !ok {
		t.Fatal("new key not tracked after expired keys were dropped")
	}
}
