package files

import (
	"math"
	"path/filepath"
	"testing"
)

func TestDiskStats(t *testing.T) {
	dir := t.TempDir()
	stats, err := diskStats(dir)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalBytes <= 0 || stats.FreeBytes < 0 || stats.FreeBytes > stats.TotalBytes {
		t.Fatalf("invalid disk statistics: %+v", stats)
	}
	if _, err := diskStats(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing directory accepted")
	}
}

func TestClampInt64(t *testing.T) {
	for _, tt := range []struct {
		input uint64
		want  int64
	}{
		{0, 0},
		{1024, 1024},
		{math.MaxInt64, math.MaxInt64},
		{math.MaxUint64, math.MaxInt64},
	} {
		if got := clampInt64(tt.input); got != tt.want {
			t.Errorf("clampInt64(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
