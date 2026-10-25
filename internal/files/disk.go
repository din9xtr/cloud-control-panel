package files

import "math"

type DiskStats struct {
	TotalBytes int64
	FreeBytes  int64
}

func clampInt64(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}
