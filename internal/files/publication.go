package files

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

type Publication struct {
	Dir      Path
	Name     string
	Size     int64
	Reserved int64
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
	target, err := s.publicationTarget(userID, stagingID, in.Dir, in.Name)
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
			return Path{}, ErrExists
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Path{}, mapFSError(err)
	} else {
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

func (s *Service) publicationTarget(userID int64, id string, dir Path, name string) (Path, error) {
	receipt := publicationFile(userID, id)
	f, err := s.volume.root.Open(receipt)
	if err == nil {
		defer f.Close()
		raw, err := io.ReadAll(io.LimitReader(f, 8193))
		if err != nil {
			return Path{}, err
		}
		var rawPath string
		if err := json.Unmarshal(raw, &rawPath); err != nil {
			return Path{}, errors.New("invalid publication receipt")
		}
		target, err := ParsePath(rawPath)
		if err != nil || len(raw) > 8192 || target.IsRoot() || target.Parent() != dir {
			return Path{}, errors.New("invalid publication receipt")
		}
		return target, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return Path{}, mapFSError(err)
	}
	target, err := s.uniquePath(userID, dir, name)
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
