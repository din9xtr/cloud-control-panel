package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/din9xtr/cloud-control-panel/internal/auth"
	"github.com/din9xtr/cloud-control-panel/internal/config"
	"github.com/din9xtr/cloud-control-panel/internal/files"
	"github.com/din9xtr/cloud-control-panel/internal/httpapi"
	"github.com/din9xtr/cloud-control-panel/internal/settings"
	"github.com/din9xtr/cloud-control-panel/internal/share"
	"github.com/din9xtr/cloud-control-panel/internal/sqlite"
	"github.com/din9xtr/cloud-control-panel/internal/upload"
	"github.com/din9xtr/cloud-control-panel/internal/web"
)

const (
	sessionTTL      = 30 * 24 * time.Hour
	sessionIdleTTL  = 7 * 24 * time.Hour
	uploadTTL       = 24 * time.Hour
	usageTTL        = 10 * time.Minute
	janitorInterval = 15 * time.Minute
	shutdownTimeout = 20 * time.Second
)

type App struct {
	cfg      config.Config
	log      *slog.Logger
	db       *sql.DB
	volume   *files.Volume
	Auth     *auth.Service
	Files    *files.Service
	Shares   *share.Service
	Uploads  *upload.Service
	Settings *settings.Service
}

func New(ctx context.Context, cfg config.Config, log *slog.Logger) (*App, error) {
	db, err := sqlite.Open(ctx, cfg.DBPath)
	if err != nil {
		return nil, err
	}
	volume, err := files.OpenVolume(cfg.StorageDir)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	clock := time.Now
	a := &App{cfg: cfg, log: log, db: db, volume: volume}

	a.Settings = settings.NewService(sqlite.NewSettings(db))
	a.Auth, err = auth.NewService(auth.Config{
		Users:       sqlite.NewUsers(db),
		Sessions:    sqlite.NewSessions(db),
		Hasher:      auth.NewBoundedHasher(auth.BcryptHasher{Cost: cfg.PasswordCost}, max(2, runtime.NumCPU()), 3*time.Second),
		IPLimiter:   auth.NewLimiter(auth.LimiterPolicy{MaxFailures: 5, Window: 15 * time.Minute, Lockout: 15 * time.Minute, MaxKeys: 10000}, clock),
		UserLimiter: auth.NewLimiter(auth.LimiterPolicy{MaxFailures: 20, Window: time.Hour, Lockout: 15 * time.Minute, MaxKeys: 10000}, clock),
		Clock:       clock,
		SessionTTL:  sessionTTL,
		IdleTTL:     sessionIdleTTL,
	})
	if err != nil {
		_ = a.Close()
		return nil, err
	}
	links := sqlite.NewLinks(db)
	observer := &linkObserver{}
	a.Files = files.NewService(files.Config{
		Volume:   volume,
		Trash:    sqlite.NewTrash(db),
		Observer: observer,
		Quota:    a.Settings,
		Clock:    clock,
		NewID:    randomID,
		UsageTTL: usageTTL,
	})
	a.Shares = share.NewService(links, a.Files, a.Settings, clock)
	observer.target = a.Shares
	a.Uploads = upload.NewService(upload.Config{
		Store:   sqlite.NewUploads(db),
		Staging: volume,
		Library: a.Files,
		Clock:   clock,
		TTL:     uploadTTL,
	})
	return a, nil
}

func (a *App) Bootstrap(ctx context.Context) error {
	required, err := a.Auth.SetupRequired(ctx)
	if err != nil {
		return err
	}
	if required {
		a.log.Warn("no account yet: open the web interface to create one")
	}
	return a.ensureUserDirs(ctx)
}

func (a *App) ensureUserDirs(ctx context.Context) error {
	rows, err := a.db.QueryContext(ctx, `SELECT id FROM users`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		if err := a.volume.EnsureUser(id); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (a *App) Handler() (http.Handler, error) {
	webHandler, err := web.NewHandler()
	if err != nil {
		return nil, err
	}
	return httpapi.NewHandler(httpapi.Config{
		TrustedProxies: a.cfg.TrustedProxies,
		SecureCookies:  a.cfg.SecureCookies,
		Limits:         httpapi.DefaultLimits(),
	}, httpapi.Deps{
		Auth:     a.Auth,
		Files:    a.Files,
		Shares:   a.Shares,
		Uploads:  a.Uploads,
		Settings: a.Settings,
		Ping:     a.db.PingContext,
		Web:      webHandler,
		Log:      a.log,
		Clock:    time.Now,
	}), nil
}

func (a *App) Run(ctx context.Context) error {
	handler, err := a.Handler()
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              a.cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
		ErrorLog:          slog.NewLogLogger(a.log.Handler(), slog.LevelWarn),
	}
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		a.log.Info("listening", slog.String("addr", a.cfg.Addr))
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	g.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})
	g.Go(func() error {
		a.runJanitor(ctx)
		return nil
	})
	return g.Wait()
}

func (a *App) runJanitor(ctx context.Context) {
	ticker := time.NewTicker(janitorInterval)
	defer ticker.Stop()
	for {
		a.sweep(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *App) sweep(ctx context.Context) {
	if _, err := a.Auth.PurgeExpiredSessions(ctx); err != nil && ctx.Err() == nil {
		a.log.Warn("purge sessions", slog.Any("error", err))
	}
	if _, err := a.Shares.PurgeExpired(ctx); err != nil && ctx.Err() == nil {
		a.log.Warn("purge links", slog.Any("error", err))
	}
	if _, err := a.Uploads.PurgeExpired(ctx); err != nil && ctx.Err() == nil {
		a.log.Warn("purge uploads", slog.Any("error", err))
	}
	retention, err := a.Settings.TrashRetention(ctx)
	if err != nil {
		return
	}
	if _, err := a.Files.PurgeTrash(ctx, retention); err != nil && ctx.Err() == nil {
		a.log.Warn("purge trash", slog.Any("error", err))
	}
}

func (a *App) Close() error {
	return errors.Join(a.volume.Close(), a.db.Close())
}

type linkObserver struct {
	target files.PathObserver
}

func (o *linkObserver) PathMoved(ctx context.Context, userID int64, from, to files.Path) error {
	return o.target.PathMoved(ctx, userID, from, to)
}

func (o *linkObserver) PathRemoved(ctx context.Context, userID int64, p files.Path) error {
	return o.target.PathRemoved(ctx, userID, p)
}

func randomID() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
