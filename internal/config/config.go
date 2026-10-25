package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"github.com/din9xtr/cloud-control-panel/internal/httpapi"
)

const defaultPasswordCost = 12

type Config struct {
	Addr           string
	DBPath         string
	StorageDir     string
	TrustedProxies []netip.Prefix
	SecureCookies  httpapi.CookiePolicy
	LogLevel       slog.Level
	PasswordCost   int
}

func Load(getenv func(string) string) (Config, error) {
	dataDir := valueOr(getenv("CLOUD_DATA_DIR"), "data")
	cfg := Config{
		Addr:         valueOr(getenv("CLOUD_ADDR"), ":8080"),
		DBPath:       valueOr(getenv("CLOUD_DB_PATH"), filepath.Join(dataDir, "cloud.db")),
		StorageDir:   valueOr(getenv("CLOUD_STORAGE_DIR"), filepath.Join(dataDir, "storage")),
		PasswordCost: defaultPasswordCost,
	}
	var errs []error
	for raw := range strings.SplitSeq(getenv("CLOUD_TRUSTED_PROXIES"), ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		prefix, err := parsePrefix(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("CLOUD_TRUSTED_PROXIES: %w", err))
			continue
		}
		cfg.TrustedProxies = append(cfg.TrustedProxies, prefix)
	}
	switch strings.ToLower(valueOr(getenv("CLOUD_SECURE_COOKIES"), "auto")) {
	case "auto":
		cfg.SecureCookies = httpapi.CookiesAuto
	case "true", "1", "yes":
		cfg.SecureCookies = httpapi.CookiesAlwaysSecure
	case "false", "0", "no":
		cfg.SecureCookies = httpapi.CookiesNeverSecure
	default:
		errs = append(errs, errors.New("CLOUD_SECURE_COOKIES: must be auto, true or false"))
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(valueOr(getenv("CLOUD_LOG_LEVEL"), "info"))); err != nil {
		errs = append(errs, fmt.Errorf("CLOUD_LOG_LEVEL: %w", err))
	}
	return cfg, errors.Join(errs...)
}

func FromEnv() (Config, error) {
	return Load(os.Getenv)
}

func parsePrefix(raw string) (netip.Prefix, error) {
	if strings.Contains(raw, "/") {
		return netip.ParsePrefix(raw)
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(addr, addr.BitLen()), nil
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
