package files

import (
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"sync"
	"time"
)

const largestFilesLimit = 10

type FolderUsage struct {
	Name  string
	Bytes int64
}

type Usage struct {
	FilesBytes int64
	TrashBytes int64
	FileCount  int
	ByFolder   []FolderUsage
	ByCategory map[Category]int64
	Largest    []Entry
}

func (u Usage) UsedBytes() int64 {
	return u.FilesBytes + u.TrashBytes
}

type Capacity struct {
	QuotaBytes int64
	UsedBytes  int64
	Disk       DiskStats
}

const diskSafetyMargin = 64 << 20

func (s *Service) Usage(ctx context.Context, userID int64) (Usage, error) {
	return s.usage.get(ctx, userID)
}

func (s *Service) Capacity(ctx context.Context, userID int64) (Capacity, error) {
	quota, err := s.quota.QuotaBytes(ctx)
	if err != nil {
		return Capacity{}, fmt.Errorf("quota: %w", err)
	}
	usage, err := s.Usage(ctx, userID)
	if err != nil {
		return Capacity{}, err
	}
	disk, err := s.Disk()
	if err != nil {
		return Capacity{}, err
	}
	return Capacity{QuotaBytes: quota, UsedBytes: usage.UsedBytes(), Disk: disk}, nil
}

func (s *Service) EnsureCapacity(ctx context.Context, userID, need, reserved int64) error {
	c, err := s.Capacity(ctx, userID)
	if err != nil {
		return err
	}
	if c.QuotaBytes > 0 && c.UsedBytes+reserved+need > c.QuotaBytes {
		return ErrQuotaExceeded
	}
	if c.Disk.FreeBytes < need+reserved+diskSafetyMargin {
		return ErrDiskFull
	}
	return nil
}

func (s *Service) Disk() (DiskStats, error) {
	return diskStats(s.volume.dir)
}

func (s *Service) computeUsage(ctx context.Context, userID int64) (Usage, error) {
	fsys, err := s.volume.userFS(userID)
	if err != nil {
		return Usage{}, err
	}
	usage := Usage{ByCategory: make(map[Category]int64)}
	folders := make(map[string]int64)
	err = fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if name == legacyTus {
			return fs.SkipDir
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		size := info.Size()
		usage.FilesBytes += size
		usage.FileCount++
		usage.ByCategory[CategoryOf(d.Name())] += size
		top, _, nested := strings.Cut(name, "/")
		if !nested {
			top = ""
		}
		folders[top] += size
		if p, perr := ParsePath(name); perr == nil {
			usage.Largest = keepLargest(usage.Largest, newEntry(p, info))
		}
		return nil
	})
	if err != nil {
		return Usage{}, fmt.Errorf("walk files: %w", err)
	}
	if usage.TrashBytes, err = s.treeSize(userTrashDir(userID)); err != nil {
		return Usage{}, err
	}
	usage.ByFolder = make([]FolderUsage, 0, len(folders))
	for name, bytes := range folders {
		usage.ByFolder = append(usage.ByFolder, FolderUsage{Name: name, Bytes: bytes})
	}
	slices.SortFunc(usage.ByFolder, func(a, b FolderUsage) int {
		return cmp.Or(cmp.Compare(b.Bytes, a.Bytes), cmp.Compare(a.Name, b.Name))
	})
	return usage, nil
}

func (s *Service) treeSize(dir string) (int64, error) {
	var total int64
	err := fs.WalkDir(s.volume.root.FS(), dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		if info, ierr := d.Info(); ierr == nil {
			total += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("measure tree: %w", err)
	}
	return total, nil
}

func keepLargest(top []Entry, e Entry) []Entry {
	if len(top) == largestFilesLimit && e.Size <= top[len(top)-1].Size {
		return top
	}
	i, _ := slices.BinarySearchFunc(top, e.Size, func(x Entry, size int64) int { return cmp.Compare(size, x.Size) })
	top = slices.Insert(top, i, e)
	if len(top) > largestFilesLimit {
		top = top[:largestFilesLimit]
	}
	return top
}

type usageEntry struct {
	mu         sync.Mutex
	value      Usage
	computedAt time.Time
	generation uint64
	valid      bool
}

type usageCache struct {
	ttl         time.Duration
	clock       func() time.Time
	compute     func(context.Context, int64) (Usage, error)
	mu          sync.Mutex
	entries     map[int64]*usageEntry
	generations map[int64]uint64
}

func newUsageCache(ttl time.Duration, clock func() time.Time, compute func(context.Context, int64) (Usage, error)) *usageCache {
	return &usageCache{
		ttl:         ttl,
		clock:       clock,
		compute:     compute,
		entries:     make(map[int64]*usageEntry),
		generations: make(map[int64]uint64),
	}
}

func (c *usageCache) get(ctx context.Context, userID int64) (Usage, error) {
	c.mu.Lock()
	entry, ok := c.entries[userID]
	if !ok {
		entry = &usageEntry{}
		c.entries[userID] = entry
	}
	c.mu.Unlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()

	generation := c.generation(userID)
	if entry.valid && entry.generation == generation && c.clock().Sub(entry.computedAt) < c.ttl {
		return entry.value, nil
	}
	value, err := c.compute(ctx, userID)
	if err != nil {
		return Usage{}, err
	}
	entry.value, entry.computedAt, entry.generation, entry.valid = value, c.clock(), generation, true
	return value, nil
}

func (c *usageCache) generation(userID int64) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generations[userID]
}

func (c *usageCache) invalidate(userID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generations[userID]++
}
