package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/din9xtr/cloud-control-panel/internal/auth"
)

const (
	sessionCookie   = "cloud_session"
	ipv6NetworkBits = 64
)

const appCSP = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; " +
	"media-src 'self'; font-src 'self'; connect-src 'self'; frame-src 'self'; object-src 'none'; worker-src 'none'; manifest-src 'self'; " +
	"base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

type principalKey struct{}

func principalFrom(ctx context.Context) auth.Principal {
	p, _ := ctx.Value(principalKey{}).(auth.Principal)
	return p
}

func (s *server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			s.fail(w, r, auth.ErrUnauthenticated)
			return
		}
		p, err := s.auth.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			if errors.Is(err, auth.ErrUnauthenticated) {
				s.clearSessionCookie(w, r)
			}
			s.fail(w, r, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	}
}

func (s *server) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   s.isSecure(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.isSecure(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *server) isSecure(r *http.Request) bool {
	switch s.cfg.SecureCookies {
	case CookiesAlwaysSecure:
		return true
	case CookiesNeverSecure:
		return false
	}
	if r.TLS != nil {
		return true
	}
	return s.fromTrustedProxy(r) && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (s *server) clientIP(r *http.Request) string {
	remote := remoteAddr(r)
	if !s.isTrusted(remote) {
		return remote.String()
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break
		}
		if !s.isTrusted(hop) {
			return hop.Unmap().String()
		}
	}
	return remote.String()
}

func (s *server) clientNetwork(r *http.Request) string {
	addr, err := netip.ParseAddr(s.clientIP(r))
	if err != nil || addr.Is4() {
		return addr.String()
	}
	return netip.PrefixFrom(addr, ipv6NetworkBits).Masked().String()
}

func (s *server) fromTrustedProxy(r *http.Request) bool {
	return s.isTrusted(remoteAddr(r))
}

func (s *server) isTrusted(addr netip.Addr) bool {
	for _, prefix := range s.cfg.TrustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func remoteAddr(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}

func (s *server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		if s.isSecure(r) {
			h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		}
		h.Set("X-Permitted-Cross-Domain-Policies", "none")
		h.Set("Content-Security-Policy", appCSP)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		next.ServeHTTP(w, r)
	})
}

func (s *server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec)
			}
			s.log.ErrorContext(r.Context(), "panic", slog.Any("panic", rec), slog.String("route", r.Pattern))
			writeError(w, http.StatusInternalServerError, "internal", "internal server error")
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *server) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		route := r.Pattern
		if route == "" {
			route = r.Method + " unmatched"
		}
		s.log.LogAttrs(r.Context(), slog.LevelInfo, "http",
			slog.String("route", route),
			slog.Int("status", rec.status),
			slog.Int64("bytes", rec.bytes),
			slog.Duration("duration", time.Since(start)),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int64
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status, r.wroteHeader = code, true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	r.wroteHeader = true
	n, err := r.ResponseWriter.Write(p)
	r.bytes += int64(n)
	return n, err
}

func (r *statusRecorder) ReadFrom(src io.Reader) (int64, error) {
	r.wroteHeader = true
	if rf, ok := r.ResponseWriter.(io.ReaderFrom); ok {
		n, err := rf.ReadFrom(src)
		r.bytes += n
		return n, err
	}
	n, err := io.Copy(struct{ io.Writer }{r.ResponseWriter}, src)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
