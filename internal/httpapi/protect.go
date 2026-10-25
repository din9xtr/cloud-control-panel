package httpapi

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

var errBusy = errors.New("server is busy, try again shortly")

type RateRule struct {
	PerSecond float64
	Burst     int
}

type Limits struct {
	MaxInFlight      int
	MaxInFlightPerIP int
	MaxVisitors      int
	General          RateRule
	Auth             RateRule
	Public           RateRule
	ReadTimeout      time.Duration
	UploadIdle       time.Duration
	Archives         int
	Searches         int
	SearchTimeout    time.Duration
}

func DefaultLimits() Limits {
	return Limits{
		MaxInFlight:      512,
		MaxInFlightPerIP: 32,
		MaxVisitors:      10000,
		General:          RateRule{PerSecond: 30, Burst: 120},
		Auth:             RateRule{PerSecond: 0.2, Burst: 10},
		Public:           RateRule{PerSecond: 2, Burst: 20},
		ReadTimeout:      30 * time.Second,
		UploadIdle:       60 * time.Second,
		Archives:         2,
		Searches:         4,
		SearchTimeout:    15 * time.Second,
	}
}

type visitor struct {
	limiter *rate.Limiter
	active  int
	seen    time.Time
}

type visitors struct {
	rule  RateRule
	max   int
	clock func() time.Time
	mu    sync.Mutex
	byIP  map[string]*visitor
}

func newVisitors(rule RateRule, maxVisitors int, clock func() time.Time) *visitors {
	return &visitors{rule: rule, max: maxVisitors, clock: clock, byIP: make(map[string]*visitor)}
}

func (v *visitors) allow(ip string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, ok := v.lookup(ip)
	if !ok {
		return false
	}
	return entry.limiter.AllowN(entry.seen, 1)
}

func (v *visitors) enter(ip string, limit int) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	entry, ok := v.lookup(ip)
	if !ok || entry.active >= limit {
		return false
	}
	entry.active++
	return true
}

func (v *visitors) leave(ip string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if entry, ok := v.byIP[ip]; ok && entry.active > 0 {
		entry.active--
	}
}

func (v *visitors) lookup(ip string) (*visitor, bool) {
	now := v.clock()
	if entry, ok := v.byIP[ip]; ok {
		entry.seen = now
		return entry, true
	}
	if len(v.byIP) >= v.max && !v.evict(now) {
		return nil, false
	}
	entry := &visitor{limiter: rate.NewLimiter(rate.Limit(v.rule.PerSecond), v.rule.Burst), seen: now}
	v.byIP[ip] = entry
	return entry, true
}

func (v *visitors) evict(now time.Time) bool {
	idle := time.Duration(float64(v.rule.Burst)/v.rule.PerSecond*float64(time.Second)) + time.Minute
	for ip, entry := range v.byIP {
		if entry.active == 0 && now.Sub(entry.seen) > idle {
			delete(v.byIP, ip)
		}
	}
	return len(v.byIP) < v.max
}

func (v *visitors) retryAfter() string {
	return strconv.Itoa(int(math.Ceil(1 / v.rule.PerSecond)))
}

type guard struct {
	limits   Limits
	inFlight chan struct{}
	general  *visitors
	auth     *visitors
	public   *visitors
	archives chan struct{}
	searches chan struct{}
}

func newGuard(limits Limits, clock func() time.Time) *guard {
	return &guard{
		limits:   limits,
		inFlight: make(chan struct{}, limits.MaxInFlight),
		general:  newVisitors(limits.General, limits.MaxVisitors, clock),
		auth:     newVisitors(limits.Auth, limits.MaxVisitors, clock),
		public:   newVisitors(limits.Public, limits.MaxVisitors, clock),
		archives: make(chan struct{}, limits.Archives),
		searches: make(chan struct{}, limits.Searches),
	}
}

func (s *server) shield(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		select {
		case s.guard.inFlight <- struct{}{}:
			defer func() { <-s.guard.inFlight }()
		default:
			rejectBusy(w)
			return
		}
		network := s.clientNetwork(r)
		if !s.guard.general.allow(network) {
			rejectTooMany(w, s.guard.general.retryAfter())
			return
		}
		if !s.guard.general.enter(network, s.guard.limits.MaxInFlightPerIP) {
			rejectTooMany(w, "1")
			return
		}
		defer s.guard.general.leave(network)
		if !isUploadWrite(r) {
			_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(s.guard.limits.ReadTimeout))
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) throttle(table *visitors, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !table.allow(s.clientNetwork(r)) {
			rejectTooMany(w, table.retryAfter())
			return
		}
		next(w, r)
	}
}

func isUploadWrite(r *http.Request) bool {
	return r.Method == http.MethodPatch
}

func rejectTooMany(w http.ResponseWriter, retryAfter string) {
	w.Header().Set("Retry-After", retryAfter)
	writeError(w, http.StatusTooManyRequests, "too_many_requests", "too many requests, slow down")
}

func rejectBusy(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "2")
	writeError(w, http.StatusServiceUnavailable, "busy", errBusy.Error())
}

func acquire(ctx context.Context, slots chan struct{}, wait time.Duration) (func(), error) {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case slots <- struct{}{}:
		return func() { <-slots }, nil
	case <-timer.C:
		return nil, errBusy
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type idleReader struct {
	body    io.Reader
	control *http.ResponseController
	idle    time.Duration
}

func (r *idleReader) Read(p []byte) (int, error) {
	_ = r.control.SetReadDeadline(time.Now().Add(r.idle))
	return r.body.Read(p)
}
