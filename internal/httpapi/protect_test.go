package httpapi

import (
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"
)

func testServer(limits Limits) *server {
	return &server{
		cfg:   Config{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}},
		log:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		clock: time.Now,
		guard: newGuard(limits, time.Now),
	}
}

func request(ip string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/files", nil)
	r.RemoteAddr = "127.0.0.1:5000"
	r.Header.Set("X-Forwarded-For", ip)
	return r
}

func ok(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func TestShieldRateLimitsPerClientIP(t *testing.T) {
	limits := DefaultLimits()
	limits.General = RateRule{PerSecond: 0.001, Burst: 3}
	s := testServer(limits)
	h := s.shield(http.HandlerFunc(ok))

	for i := range 3 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, request("203.0.113.7"))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("request %d status = %d", i, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, request("203.0.113.7"))
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("over limit status = %d, Retry-After = %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	other := httptest.NewRecorder()
	h.ServeHTTP(other, request("198.51.100.9"))
	if other.Code != http.StatusNoContent {
		t.Fatalf("another client was limited: %d", other.Code)
	}
}

func TestShieldCapsConcurrencyAndShedsLoad(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxInFlight = 2
	limits.MaxInFlightPerIP = 1
	s := testServer(limits)
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	h := s.shield(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		started <- struct{}{}
		<-release
		w.WriteHeader(http.StatusNoContent)
	}))

	var wg sync.WaitGroup
	for _, ip := range []string{"203.0.113.1", "203.0.113.2"} {
		wg.Go(func() { h.ServeHTTP(httptest.NewRecorder(), request(ip)) })
	}
	<-started
	<-started

	busy := httptest.NewRecorder()
	h.ServeHTTP(busy, request("203.0.113.3"))
	if busy.Code != http.StatusServiceUnavailable {
		t.Fatalf("global overload status = %d", busy.Code)
	}
	close(release)
	wg.Wait()

	limits.MaxInFlight = 10
	s = testServer(limits)
	block := make(chan struct{})
	entered := make(chan struct{})
	h = s.shield(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(entered)
		<-block
	}))
	go h.ServeHTTP(httptest.NewRecorder(), request("203.0.113.4"))
	<-entered
	second := httptest.NewRecorder()
	h.ServeHTTP(second, request("203.0.113.4"))
	close(block)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("per-IP concurrency status = %d", second.Code)
	}
}

func TestThrottleUsesItsOwnBudget(t *testing.T) {
	limits := DefaultLimits()
	limits.Auth = RateRule{PerSecond: 0.001, Burst: 1}
	s := testServer(limits)
	h := s.throttle(s.guard.auth, ok)
	first, second := httptest.NewRecorder(), httptest.NewRecorder()
	h(first, request("203.0.113.8"))
	h(second, request("203.0.113.8"))
	if first.Code != http.StatusNoContent || second.Code != http.StatusTooManyRequests {
		t.Fatalf("statuses = %d, %d", first.Code, second.Code)
	}
}

func TestVisitorTableIsBounded(t *testing.T) {
	v := newVisitors(RateRule{PerSecond: 1, Burst: 1}, 2, time.Now)
	if !v.allow("a") || !v.allow("b") {
		t.Fatal("first visitors rejected")
	}
	if v.allow("c") {
		t.Fatal("table grew past its limit while entries are fresh")
	}
	if len(v.byIP) != 2 {
		t.Fatalf("visitors = %d", len(v.byIP))
	}
}

func TestSlowRequestBodyIsCutOff(t *testing.T) {
	limits := DefaultLimits()
	limits.ReadTimeout = 200 * time.Millisecond
	s := testServer(limits)
	readErr := make(chan error, 1)
	srv := httptest.NewServer(s.shield(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		readErr <- err
	})))
	defer srv.Close()

	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = conn.Write([]byte("POST /api/v1/auth/login HTTP/1.1\r\nHost: x\r\nContent-Length: 100\r\n\r\n{"))
	select {
	case err := <-readErr:
		if err == nil {
			t.Fatal("slow body was read completely")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("slow body was not cut off")
	}
}

func TestShieldGroupsIPv6ClientsByNetwork(t *testing.T) {
	limits := DefaultLimits()
	limits.General = RateRule{PerSecond: 0.001, Burst: 2}
	s := testServer(limits)
	h := s.shield(http.HandlerFunc(ok))

	for i, ip := range []string{"2001:db8:1:2::a", "2001:db8:1:2::b"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, request(ip))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("request %d status = %d", i, rec.Code)
		}
	}
	rotated := httptest.NewRecorder()
	h.ServeHTTP(rotated, request("2001:db8:1:2:ffff::1"))
	if rotated.Code != http.StatusTooManyRequests {
		t.Fatalf("address rotation inside a /64 status = %d", rotated.Code)
	}
	neighbor := httptest.NewRecorder()
	h.ServeHTTP(neighbor, request("2001:db8:1:3::1"))
	if neighbor.Code != http.StatusNoContent {
		t.Fatalf("another network was limited: %d", neighbor.Code)
	}
}
