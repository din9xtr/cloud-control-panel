//go:build !windows

package files

import (
	"fmt"

	"golang.org/x/sys/unix"
)

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
