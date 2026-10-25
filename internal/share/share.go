package share

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/din9xtr/cloud-control-panel/internal/files"
)

const (
	MinTTL        = 5 * time.Minute
	MaxTTL        = 30 * 24 * time.Hour
	tokenBytes    = 16
	idBytes       = 8
	MaxUploadSize = int64(5) << 30
)

type Mode string

const (
	ModeDownload Mode = "download"
	ModeUpload   Mode = "upload"
)

var (
	ErrLinkNotFound = errors.New("link not found")
	ErrInvalidTTL   = errors.New("link lifetime must be between 5 minutes and 30 days")
	ErrNotAFile     = errors.New("only files can be shared")
	ErrInvalidMode  = errors.New("invalid link mode; upload links require a folder")
)

type Link struct {
	Mode      Mode
	Kind      files.Kind
	ID        string
	UserID    int64
	Path      files.Path
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
	Downloads int64
}

type Store interface {
	CreateLink(ctx context.Context, l Link) error
	LinkByToken(ctx context.Context, token string) (Link, error)
	ActiveLinks(ctx context.Context, userID int64, now time.Time) ([]Link, error)
	DeleteLink(ctx context.Context, userID int64, id string) error
	IncrementDownloads(ctx context.Context, id string) error
	MoveLinks(ctx context.Context, userID int64, from, to files.Path) error
	DeleteLinksUnder(ctx context.Context, userID int64, p files.Path) error
	DeleteExpiredLinks(ctx context.Context, now time.Time) (int64, error)
}

type FileSource interface {
	Stat(ctx context.Context, userID int64, p files.Path) (files.Entry, error)
	Open(ctx context.Context, userID int64, p files.Path) (*os.File, files.Entry, error)
}

type DefaultTTL interface {
	DefaultLinkTTL(ctx context.Context) (time.Duration, error)
}

type Service struct {
	store      Store
	files      FileSource
	defaultTTL DefaultTTL
	clock      func() time.Time
}

var _ files.PathObserver = (*Service)(nil)

func NewService(store Store, source FileSource, defaultTTL DefaultTTL, clock func() time.Time) *Service {
	return &Service{store: store, files: source, defaultTTL: defaultTTL, clock: clock}
}

func (s *Service) Create(ctx context.Context, userID int64, p files.Path, ttl time.Duration) (Link, error) {
	return s.CreateWithMode(ctx, userID, p, ttl, ModeDownload)
}

func (s *Service) CreateWithMode(ctx context.Context, userID int64, p files.Path, ttl time.Duration, mode Mode) (Link, error) {
	if mode == "" {
		mode = ModeDownload
	}
	if mode != ModeDownload && mode != ModeUpload {
		return Link{}, ErrInvalidMode
	}
	if p.IsRoot() {
		return Link{}, files.ErrRootOperation
	}
	if ttl == 0 {
		var err error
		if ttl, err = s.defaultTTL.DefaultLinkTTL(ctx); err != nil {
			return Link{}, fmt.Errorf("default ttl: %w", err)
		}
	}
	if ttl < MinTTL || ttl > MaxTTL {
		return Link{}, ErrInvalidTTL
	}
	entry, err := s.files.Stat(ctx, userID, p)
	if err != nil {
		return Link{}, err
	}
	if mode == ModeUpload && entry.Kind != files.KindDir {
		return Link{}, ErrInvalidMode
	}
	token, err := random(tokenBytes, base64.RawURLEncoding.EncodeToString)
	if err != nil {
		return Link{}, err
	}
	id, err := random(idBytes, hex.EncodeToString)
	if err != nil {
		return Link{}, err
	}
	now := s.clock().UTC()
	link := Link{ID: id, UserID: userID, Path: p, Token: token, CreatedAt: now, ExpiresAt: now.Add(ttl), Mode: mode, Kind: entry.Kind}
	if err := s.store.CreateLink(ctx, link); err != nil {
		return Link{}, fmt.Errorf("store link: %w", err)
	}
	return link, nil
}

func (s *Service) List(ctx context.Context, userID int64) ([]Link, error) {
	return s.store.ActiveLinks(ctx, userID, s.clock())
}

func (s *Service) Revoke(ctx context.Context, userID int64, id string) error {
	return s.store.DeleteLink(ctx, userID, id)
}

func (s *Service) Open(ctx context.Context, token string) (*os.File, files.Entry, Link, error) {
	link, err := s.Resolve(ctx, token)
	if err != nil {
		return nil, files.Entry{}, Link{}, err
	}
	if link.Mode != ModeDownload || link.Kind != files.KindFile {
		return nil, files.Entry{}, Link{}, ErrLinkNotFound
	}
	f, entry, err := s.files.Open(ctx, link.UserID, link.Path)
	if errors.Is(err, files.ErrNotFound) || errors.Is(err, files.ErrIsDirectory) {
		return nil, files.Entry{}, Link{}, ErrLinkNotFound
	}
	if err != nil {
		return nil, files.Entry{}, Link{}, err
	}
	return f, entry, link, nil
}

func (s *Service) Resolve(ctx context.Context, token string) (Link, error) {
	link, err := s.store.LinkByToken(ctx, token)
	if err != nil {
		return Link{}, err
	}
	if !s.clock().Before(link.ExpiresAt) {
		return Link{}, ErrLinkNotFound
	}
	entry, err := s.files.Stat(ctx, link.UserID, link.Path)
	if errors.Is(err, files.ErrNotFound) || err == nil && entry.Kind != link.Kind {
		return Link{}, ErrLinkNotFound
	}
	if err != nil {
		return Link{}, err
	}
	return link, nil
}

func (s *Service) ResolvePath(ctx context.Context, token, relative string) (Link, files.Path, error) {
	link, err := s.Resolve(ctx, token)
	if err != nil {
		return Link{}, files.Path{}, err
	}
	if link.Mode != ModeDownload || link.Kind != files.KindDir {
		return Link{}, files.Path{}, ErrLinkNotFound
	}
	rel, err := files.ParsePath(relative)
	if err != nil {
		return Link{}, files.Path{}, err
	}
	return link, rel, nil
}

func (s *Service) RecordDownload(ctx context.Context, id string) error {
	return s.store.IncrementDownloads(ctx, id)
}

func (s *Service) PurgeExpired(ctx context.Context) (int64, error) {
	return s.store.DeleteExpiredLinks(ctx, s.clock())
}

func (s *Service) PathMoved(ctx context.Context, userID int64, from, to files.Path) error {
	return s.store.MoveLinks(ctx, userID, from, to)
}

func (s *Service) PathRemoved(ctx context.Context, userID int64, p files.Path) error {
	return s.store.DeleteLinksUnder(ctx, userID, p)
}

func random(n int, encode func([]byte) string) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	return encode(buf), nil
}
