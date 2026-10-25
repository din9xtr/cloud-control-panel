package settings

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"
)

const (
	keyQuotaBytes     = "quota_bytes"
	keyTrashRetention = "trash_retention_seconds"
	keyDefaultLinkTTL = "default_link_ttl_seconds"

	maxQuotaBytes     = int64(1) << 50
	minTrashRetention = 24 * time.Hour
	maxTrashRetention = 365 * 24 * time.Hour
	minLinkTTL        = time.Hour
	maxLinkTTL        = 30 * 24 * time.Hour
)

var (
	ErrInvalidQuota          = errors.New("quota must be between 0 and 1 PiB")
	ErrInvalidTrashRetention = errors.New("trash retention must be between 1 and 365 days")
	ErrInvalidLinkTTL        = errors.New("default link lifetime must be between 1 hour and 30 days")
	ErrCorrupt               = errors.New("stored settings are missing or invalid")
)

type Settings struct {
	QuotaBytes     int64
	TrashRetention time.Duration
	DefaultLinkTTL time.Duration
}

func (s Settings) Validate() error {
	switch {
	case s.QuotaBytes < 0 || s.QuotaBytes > maxQuotaBytes:
		return ErrInvalidQuota
	case s.TrashRetention < minTrashRetention || s.TrashRetention > maxTrashRetention:
		return ErrInvalidTrashRetention
	case s.DefaultLinkTTL < minLinkTTL || s.DefaultLinkTTL > maxLinkTTL:
		return ErrInvalidLinkTTL
	}
	return nil
}

type Store interface {
	LoadSettings(ctx context.Context) (map[string]string, error)
	SaveSettings(ctx context.Context, values map[string]string) error
}

type Service struct {
	store  Store
	mu     sync.RWMutex
	cached *Settings
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Get(ctx context.Context) (Settings, error) {
	s.mu.RLock()
	cached := s.cached
	s.mu.RUnlock()
	if cached != nil {
		return *cached, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil {
		return *s.cached, nil
	}
	values, err := s.store.LoadSettings(ctx)
	if err != nil {
		return Settings{}, fmt.Errorf("load settings: %w", err)
	}
	loaded, err := parse(values)
	if err != nil {
		return Settings{}, err
	}
	s.cached = &loaded
	return loaded, nil
}

func (s *Service) Update(ctx context.Context, next Settings) (Settings, error) {
	if err := next.Validate(); err != nil {
		return Settings{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.store.SaveSettings(ctx, map[string]string{
		keyQuotaBytes:     strconv.FormatInt(next.QuotaBytes, 10),
		keyTrashRetention: strconv.FormatInt(int64(next.TrashRetention/time.Second), 10),
		keyDefaultLinkTTL: strconv.FormatInt(int64(next.DefaultLinkTTL/time.Second), 10),
	})
	if err != nil {
		return Settings{}, fmt.Errorf("save settings: %w", err)
	}
	s.cached = &next
	return next, nil
}

func (s *Service) QuotaBytes(ctx context.Context) (int64, error) {
	cfg, err := s.Get(ctx)
	return cfg.QuotaBytes, err
}

func (s *Service) DefaultLinkTTL(ctx context.Context) (time.Duration, error) {
	cfg, err := s.Get(ctx)
	return cfg.DefaultLinkTTL, err
}

func (s *Service) TrashRetention(ctx context.Context) (time.Duration, error) {
	cfg, err := s.Get(ctx)
	return cfg.TrashRetention, err
}

func parse(values map[string]string) (Settings, error) {
	quota, qerr := intValue(values, keyQuotaBytes)
	retention, rerr := secondsValue(values, keyTrashRetention)
	linkTTL, lerr := secondsValue(values, keyDefaultLinkTTL)
	if err := errors.Join(qerr, rerr, lerr); err != nil {
		return Settings{}, err
	}
	loaded := Settings{QuotaBytes: quota, TrashRetention: retention, DefaultLinkTTL: linkTTL}
	if err := loaded.Validate(); err != nil {
		return Settings{}, fmt.Errorf("%w: %w", ErrCorrupt, err)
	}
	return loaded, nil
}

func intValue(values map[string]string, key string) (int64, error) {
	raw, ok := values[key]
	if !ok {
		return 0, fmt.Errorf("%w: %s is not set", ErrCorrupt, key)
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %s is not a number", ErrCorrupt, key)
	}
	return v, nil
}

func secondsValue(values map[string]string, key string) (time.Duration, error) {
	v, err := intValue(values, key)
	if err != nil {
		return 0, err
	}
	if v < 0 || v > int64(maxTrashRetention/time.Second) {
		return 0, fmt.Errorf("%w: %s is out of range", ErrCorrupt, key)
	}
	return time.Duration(v) * time.Second, nil
}
