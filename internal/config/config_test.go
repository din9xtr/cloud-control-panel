package config

import (
	"log/slog"
	"testing"

	"github.com/din9xtr/cloud-control-panel/internal/httpapi"
)

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Addr != ":8080" || cfg.DBPath != "data/cloud.db" || cfg.StorageDir != "data/storage" {
		t.Fatalf("defaults = %+v", cfg)
	}
	if cfg.SecureCookies != httpapi.CookiesAuto || cfg.LogLevel != slog.LevelInfo {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestLoadParsesValues(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"CLOUD_TRUSTED_PROXIES": "127.0.0.1, 10.0.0.0/8",
		"CLOUD_SECURE_COOKIES":  "true",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0].Bits() != 32 {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoadReportsAllErrors(t *testing.T) {
	_, err := Load(env(map[string]string{
		"CLOUD_TRUSTED_PROXIES": "nope",
		"CLOUD_SECURE_COOKIES":  "maybe",
	}))
	if err == nil {
		t.Fatal("expected errors")
	}
}
