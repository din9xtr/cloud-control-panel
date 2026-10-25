package settings

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mapStore struct {
	values map[string]string
	loads  int
}

func (m *mapStore) LoadSettings(context.Context) (map[string]string, error) {
	m.loads++
	return m.values, nil
}

func (m *mapStore) SaveSettings(_ context.Context, values map[string]string) error {
	for k, v := range values {
		m.values[k] = v
	}
	return nil
}

func storedDefaults() map[string]string {
	return map[string]string{
		keyQuotaBytes:     "0",
		keyTrashRetention: "2592000",
		keyDefaultLinkTTL: "86400",
	}
}

func TestGetLoadsStoredValuesAndCaches(t *testing.T) {
	values := storedDefaults()
	values[keyQuotaBytes] = "1024"
	store := &mapStore{values: values}
	svc := NewService(store)
	got, err := svc.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := Settings{QuotaBytes: 1024, TrashRetention: 30 * 24 * time.Hour, DefaultLinkTTL: 24 * time.Hour}
	if got != want {
		t.Fatalf("Get() = %+v, want %+v", got, want)
	}
	_, _ = svc.Get(context.Background())
	if store.loads != 1 {
		t.Fatalf("store loaded %d times", store.loads)
	}
}

func TestGetRejectsCorruptValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		drop  bool
	}{
		{name: "missing key", key: keyTrashRetention, drop: true},
		{name: "not a number", key: keyQuotaBytes, value: "lots"},
		{name: "out of range", key: keyDefaultLinkTTL, value: "60"},
		{name: "overflow", key: keyTrashRetention, value: "9223372036854775807"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := storedDefaults()
			if tt.drop {
				delete(values, tt.key)
			} else {
				values[tt.key] = tt.value
			}
			if _, err := NewService(&mapStore{values: values}).Get(context.Background()); !errors.Is(err, ErrCorrupt) {
				t.Fatalf("Get() error = %v, want ErrCorrupt", err)
			}
		})
	}
}

func TestUpdatePersistsAndValidates(t *testing.T) {
	store := &mapStore{values: storedDefaults()}
	svc := NewService(store)
	ctx := context.Background()

	bad := Settings{QuotaBytes: 0, TrashRetention: 30 * 24 * time.Hour, DefaultLinkTTL: time.Minute}
	if _, err := svc.Update(ctx, bad); !errors.Is(err, ErrInvalidLinkTTL) {
		t.Fatalf("Update() error = %v", err)
	}
	next := Settings{QuotaBytes: 70 << 30, TrashRetention: 7 * 24 * time.Hour, DefaultLinkTTL: 7 * 24 * time.Hour}
	if _, err := svc.Update(ctx, next); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewService(store).Get(ctx)
	if err != nil || reloaded != next {
		t.Fatalf("reloaded = %+v, %v", reloaded, err)
	}
}
