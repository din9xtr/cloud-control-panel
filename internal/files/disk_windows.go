package files

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func diskStats(dir string) (DiskStats, error) {
	name, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return DiskStats{}, fmt.Errorf("disk directory: %w", err)
	}
	var available, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(name, &available, &total, &free); err != nil {
		return DiskStats{}, fmt.Errorf("disk space: %w", err)
	}
	return DiskStats{TotalBytes: clampInt64(total), FreeBytes: clampInt64(available)}, nil
}
