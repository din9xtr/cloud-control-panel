package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/din9xtr/cloud-control-panel/internal/auth"
	"github.com/din9xtr/cloud-control-panel/internal/files"
	"github.com/din9xtr/cloud-control-panel/internal/settings"
	"github.com/din9xtr/cloud-control-panel/internal/share"
	"github.com/din9xtr/cloud-control-panel/internal/upload"
)

type CookiePolicy int

const (
	CookiesAuto CookiePolicy = iota
	CookiesAlwaysSecure
	CookiesNeverSecure
)

type Config struct {
	TrustedProxies []netip.Prefix
	SecureCookies  CookiePolicy
	Limits         Limits
}

type Deps struct {
	Auth     *auth.Service
	Files    *files.Service
	Shares   *share.Service
	Uploads  *upload.Service
	Settings *settings.Service
	Ping     func(context.Context) error
	Web      http.Handler
	Log      *slog.Logger
	Clock    func() time.Time
}

type server struct {
	web      http.Handler
	cfg      Config
	auth     *auth.Service
	files    *files.Service
	shares   *share.Service
	uploads  *upload.Service
	settings *settings.Service
	ping     func(context.Context) error
	log      *slog.Logger
	clock    func() time.Time
	guard    *guard
}

func NewHandler(cfg Config, deps Deps) http.Handler {
	s := &server{
		web:      deps.Web,
		cfg:      cfg,
		auth:     deps.Auth,
		files:    deps.Files,
		shares:   deps.Shares,
		uploads:  deps.Uploads,
		settings: deps.Settings,
		ping:     deps.Ping,
		log:      deps.Log,
		clock:    deps.Clock,
		guard:    newGuard(cfg.Limits, deps.Clock),
	}
	mux := http.NewServeMux()
	s.routes(mux)
	mux.Handle("/", s.fallback(deps.Web))
	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusForbidden, "forbidden", "cross-origin request rejected")
	}))
	return s.recoverer(s.accessLog(s.securityHeaders(s.shield(protection.Handler(mux)))))
}

func (s *server) routes(mux *http.ServeMux) {
	const v1 = "/api/v1"
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /s/{token}", s.throttle(s.guard.public, s.publicDownload))
	mux.HandleFunc("GET /share/{token}", s.throttle(s.guard.public, func(w http.ResponseWriter, r *http.Request) {
		publicHeaders(w)
		if s.web == nil {
			s.missingLink(w)
			return
		}
		s.web.ServeHTTP(w, r)
	}))
	mux.HandleFunc("GET "+v1+"/public/links/{token}", s.throttle(s.guard.public, s.publicFolder))
	mux.HandleFunc("GET "+v1+"/public/links/{token}/content", s.throttle(s.guard.public, s.publicFolderContent))
	mux.HandleFunc("GET "+v1+"/public/links/{token}/archive", s.throttle(s.guard.public, s.publicFolderArchive))
	mux.HandleFunc("OPTIONS "+v1+"/public/links/{token}/uploads", s.requireUploadLink(s.tusOptions))
	mux.HandleFunc("POST "+v1+"/public/links/{token}/uploads", s.requireUploadLink(s.tusCreate))
	mux.HandleFunc("HEAD "+v1+"/public/links/{token}/uploads/{id}", s.requireUploadLink(s.tusStatus))
	mux.HandleFunc("PATCH "+v1+"/public/links/{token}/uploads/{id}", s.requireUploadLink(s.tusWrite))
	mux.HandleFunc("DELETE "+v1+"/public/links/{token}/uploads/{id}", s.requireUploadLink(s.tusTerminate))

	mux.HandleFunc("GET "+v1+"/setup", s.setupStatus)
	mux.HandleFunc("POST "+v1+"/setup", s.throttle(s.guard.auth, s.setup))
	mux.HandleFunc("POST "+v1+"/auth/login", s.throttle(s.guard.auth, s.login))
	mux.HandleFunc("POST "+v1+"/auth/logout", s.requireAuth(s.logout))
	mux.HandleFunc("GET "+v1+"/auth/me", s.requireAuth(s.me))

	mux.HandleFunc("PUT "+v1+"/account/credentials", s.requireAuth(s.changeCredentials))
	mux.HandleFunc("GET "+v1+"/account/sessions", s.requireAuth(s.listSessions))
	mux.HandleFunc("DELETE "+v1+"/account/sessions/{id}", s.requireAuth(s.revokeSession))

	mux.HandleFunc("GET "+v1+"/files", s.requireAuth(s.listFolder))
	mux.HandleFunc("GET "+v1+"/files/search", s.requireAuth(s.searchFiles))
	mux.HandleFunc("POST "+v1+"/files/folders", s.requireAuth(s.createFolder))
	mux.HandleFunc("POST "+v1+"/files/rename", s.requireAuth(s.renameEntry))
	mux.HandleFunc("POST "+v1+"/files/move", s.requireAuth(s.moveEntries))
	mux.HandleFunc("POST "+v1+"/files/trash", s.requireAuth(s.trashEntries))
	mux.HandleFunc("GET "+v1+"/files/content", s.requireAuth(s.downloadFile))
	mux.HandleFunc("GET "+v1+"/files/archive", s.requireAuth(s.downloadArchive))

	mux.HandleFunc("GET "+v1+"/trash", s.requireAuth(s.listTrash))
	mux.HandleFunc("DELETE "+v1+"/trash", s.requireAuth(s.emptyTrash))
	mux.HandleFunc("DELETE "+v1+"/trash/{id}", s.requireAuth(s.deleteTrashItem))
	mux.HandleFunc("POST "+v1+"/trash/{id}/restore", s.requireAuth(s.restoreTrashItem))

	mux.HandleFunc("GET "+v1+"/links", s.requireAuth(s.listLinks))
	mux.HandleFunc("POST "+v1+"/links", s.requireAuth(s.createLink))
	mux.HandleFunc("DELETE "+v1+"/links/{id}", s.requireAuth(s.revokeLink))

	mux.HandleFunc("GET "+v1+"/storage", s.requireAuth(s.storageUsage))
	mux.HandleFunc("GET "+v1+"/settings", s.requireAuth(s.getSettings))
	mux.HandleFunc("PUT "+v1+"/settings", s.requireAuth(s.updateSettings))

	mux.HandleFunc("OPTIONS "+v1+"/uploads", s.tusOptions)
	mux.HandleFunc("POST "+v1+"/uploads", s.requireAuth(s.tusCreate))
	mux.HandleFunc("HEAD "+v1+"/uploads/{id}", s.requireAuth(s.tusStatus))
	mux.HandleFunc("PATCH "+v1+"/uploads/{id}", s.requireAuth(s.tusWrite))
	mux.HandleFunc("DELETE "+v1+"/uploads/{id}", s.requireAuth(s.tusTerminate))
}

func (s *server) fallback(web http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if web == nil || strings.HasPrefix(r.URL.Path, "/api/") || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			writeError(w, http.StatusNotFound, "not_found", "unknown endpoint")
			return
		}
		web.ServeHTTP(w, r)
	})
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
