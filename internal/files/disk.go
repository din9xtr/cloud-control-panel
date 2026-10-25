package files

import (
	"fmt"
	"math"

	"golang.org/x/sys/unix"
)

type DiskStats struct {
	TotalBytes int64
	FreeBytes  int64
}

func diskStats(dir string) (DiskStats, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return DiskStats{}, fmt.Errorf("statfs: %w", err)
	}
	blockSize := uint64(st.Bsize)
	return DiskStats{
		TotalBytes: clampInt64(st.Blocks * blockSize),
		FreeBytes:  clampInt64(st.Bavail * blockSize),
	}, nil
}

func clampInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}
