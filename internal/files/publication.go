package files

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

const publicationReceiptLimit = 32 << 10

type Publication struct {
	Dir           Path
	Name          string
	Size          int64
	Reserved      int64
	SkipIdentical bool
	ExistingOnly  bool
}

func (s *Service) Publish(ctx context.Context, userID int64, stagingID string, in Publication) (Path, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Path{}, err
	}
	if err := s.requireDir(userID, in.Dir); err != nil {
		return Path{}, err
	}
	target, err := s.publicationTarget(userID, stagingID, in)
	if err != nil {
		return Path{}, err
	}
	stage := stagingFile(userID, stagingID)
	sourceInfo, err := s.volume.root.Stat(stage)
	if err != nil {
		return Path{}, mapFSError(err)
	}
	if !sourceInfo.Mode().IsRegular() || sourceInfo.Size() != in.Size {
		return Path{}, errors.New("staging length differs from declared upload length")
	}
	destination := userPath(userID, target)
	if info, err := s.volume.root.Lstat(destination); err == nil {
		if !os.SameFile(sourceInfo, info) {
			if !in.SkipIdentical || !info.Mode().IsRegular() || info.Size() != in.Size {
				return Path{}, ErrExists
			}
			if err := s.matchExisting(ctx, stage, destination, info); err != nil {
				return Path{}, err
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Path{}, mapFSError(err)
	} else {
		if in.ExistingOnly {
			return Path{}, ErrNotFound
		}
		if err := s.ensureQuota(ctx, userID, in.Size, in.Reserved); err != nil {
			return Path{}, err
		}
		if err := s.volume.root.Link(stage, destination); err != nil {
			return Path{}, mapFSError(err)
		}
	}
	s.usage.invalidate(userID)
	return target, nil
}

func (s *Service) publicationTarget(userID int64, id string, in Publication) (Path, error) {
	receipt := publicationFile(userID, id)
	f, err := s.volume.root.Open(receipt)
	if err == nil {
		defer f.Close()
		raw, err := io.ReadAll(io.LimitReader(f, publicationReceiptLimit+1))
		if err != nil {
			return Path{}, err
		}
		var rawPath string
		if err := json.Unmarshal(raw, &rawPath); err != nil {
			return Path{}, errors.New("invalid publication receipt")
		}
		target, err := ParsePath(rawPath)
		if err != nil || len(raw) > publicationReceiptLimit || target.IsRoot() || target.Parent() != in.Dir {
			return Path{}, errors.New("invalid publication receipt")
		}
		return target, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return Path{}, mapFSError(err)
	}
	target, err := in.Dir.Join(in.Name)
	if !in.SkipIdentical {
		target, err = s.uniquePath(userID, in.Dir, in.Name)
	}
	if err != nil {
		return Path{}, err
	}
	f, err = s.volume.root.OpenFile(receipt, os.O_CREATE|os.O_EXCL|os.O_WRONLY, filePerm)
	if err != nil {
		return Path{}, mapFSError(err)
	}
	raw, err := json.Marshal(target.String())
	if err != nil {
		_ = f.Close()
		return Path{}, err
	}
	_, writeErr := f.Write(raw)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return Path{}, fmt.Errorf("save publication receipt: %w", mapFSError(err))
	}
	return target, nil
}

func (s *Service) ensureQuota(ctx context.Context, userID, need, reserved int64) error {
	quota, err := s.quota.QuotaBytes(ctx)
	if err != nil {
		return err
	}
	if quota == 0 {
		return nil
	}
	usage, err := s.Usage(ctx, userID)
	if err != nil {
		return err
	}
	if usage.UsedBytes()+need+reserved > quota {
		return ErrQuotaExceeded
	}
	return nil
}

func (s *Service) CheckExistingUpload(ctx context.Context, userID int64, dir Path, name string, size, reserved int64) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	target, err := dir.Join(name)
	if err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	info, err := s.volume.root.Lstat(userPath(userID, target))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, mapFSError(err)
	}
	if !info.Mode().IsRegular() || info.Size() != size {
		return false, ErrExists
	}
	disk, err := s.Disk()
	if err != nil {
		return false, err
	}
	if disk.FreeBytes < size+reserved+diskSafetyMargin {
		return false, ErrDiskFull
	}
	return true, nil
}

func (s *Service) matchExisting(ctx context.Context, stage, destination string, info fs.FileInfo) error {
	source, err := s.volume.root.Open(stage)
	if err != nil {
		return mapFSError(err)
	}
	defer source.Close()
	existing, err := s.volume.root.Open(destination)
	if err != nil {
		return mapFSError(err)
	}
	defer existing.Close()
	opened, err := existing.Stat()
	if err != nil {
		return mapFSError(err)
	}
	if !os.SameFile(info, opened) {
		return ErrExists
	}
	s.mu.Unlock()
	equal, compareErr := s.compareContents(ctx, source, existing, info.Size())
	s.mu.Lock()
	if compareErr != nil {
		return compareErr
	}
	if !equal {
		return ErrExists
	}
	current, err := s.volume.root.Lstat(destination)
	if err != nil {
		return mapFSError(err)
	}
	if !os.SameFile(info, current) || current.Size() != info.Size() || !current.ModTime().Equal(info.ModTime()) {
		return ErrExists
	}
	return nil
}

func (s *Service) compareContents(ctx context.Context, a, b io.Reader, size int64) (bool, error) {
	select {
	case s.comparisons <- struct{}{}:
		defer func() { <-s.comparisons }()
		return sameContents(ctx, a, b, size)
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func sameContents(ctx context.Context, a, b io.Reader, size int64) (bool, error) {
	left, right := make([]byte, 128<<10), make([]byte, 128<<10)
	for remaining := size; remaining > 0; {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		count := int(min(int64(len(left)), remaining))
		if _, err := io.ReadFull(a, left[:count]); err != nil {
			return false, err
		}
		if _, err := io.ReadFull(b, right[:count]); err != nil {
			return false, err
		}
		if !bytes.Equal(left[:count], right[:count]) {
			return false, nil
		}
		remaining -= int64(count)
	}
	return true, ctx.Err()
}
