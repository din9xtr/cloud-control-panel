package files

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"
)

type TrashItem struct {
	ID           string
	UserID       int64
	OriginalPath Path
	Kind         Kind
	Size         int64
	TrashedAt    time.Time
}

func (t TrashItem) Name() string {
	return t.OriginalPath.Name()
}

var ErrTrashItemNotFound = errors.New("trash item not found")

func (s *Service) Trash(ctx context.Context, userID int64, paths []Path) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	defer s.usage.invalidate(userID)

	for _, p := range topLevel(paths) {
		if err := checkMutable(p); err != nil {
			return err
		}
		if err := s.moveToTrash(ctx, userID, p); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) moveToTrash(ctx context.Context, userID int64, p Path) error {
	info, err := s.volume.root.Lstat(userPath(userID, p))
	if err != nil {
		return mapFSError(err)
	}
	item := TrashItem{
		ID:           s.newID(),
		UserID:       userID,
		OriginalPath: p,
		Kind:         newEntry(p, info).Kind,
		Size:         info.Size(),
		TrashedAt:    s.clock().UTC(),
	}
	if item.Kind == KindDir {
		if item.Size, err = s.treeSize(userPath(userID, p)); err != nil {
			return err
		}
	}
	if err := s.volume.root.Rename(userPath(userID, p), trashPath(userID, item.ID)); err != nil {
		return mapFSError(err)
	}
	if err := s.trash.AddTrashItem(ctx, item); err != nil {
		if rerr := s.volume.root.Rename(trashPath(userID, item.ID), userPath(userID, p)); rerr != nil {
			return errors.Join(fmt.Errorf("record trash item: %w", err), fmt.Errorf("rollback trash: %w", rerr))
		}
		return fmt.Errorf("record trash item: %w", err)
	}
	if err := s.observer.PathRemoved(ctx, userID, p); err != nil {
		return fmt.Errorf("revoke links: %w", err)
	}
	return nil
}

func (s *Service) TrashItems(ctx context.Context, userID int64) ([]TrashItem, error) {
	return s.trash.TrashItems(ctx, userID)
}

func (s *Service) Restore(ctx context.Context, userID int64, id string) (Path, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, err := s.trash.TrashItem(ctx, userID, id)
	if err != nil {
		return Path{}, err
	}
	parent := item.OriginalPath.Parent()
	if err := s.volume.root.MkdirAll(userPath(userID, parent), dirPerm); err != nil {
		return Path{}, mapFSError(err)
	}
	target, err := s.uniquePath(userID, parent, item.Name())
	if err != nil {
		return Path{}, err
	}
	if err := s.volume.root.Rename(trashPath(userID, id), userPath(userID, target)); err != nil {
		return Path{}, mapFSError(err)
	}
	s.usage.invalidate(userID)
	if err := s.trash.DeleteTrashItem(ctx, userID, id); err != nil {
		return Path{}, fmt.Errorf("forget trash item: %w", err)
	}
	return target, nil
}

func (s *Service) DeleteForever(ctx context.Context, userID int64, id string) error {
	if _, err := s.trash.TrashItem(ctx, userID, id); err != nil {
		return err
	}
	return s.purge(ctx, userID, id)
}

func (s *Service) EmptyTrash(ctx context.Context, userID int64) error {
	items, err := s.trash.TrashItems(ctx, userID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := s.purge(ctx, userID, item.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) PurgeTrash(ctx context.Context, retention time.Duration) (int, error) {
	items, err := s.trash.ExpiredTrashItems(ctx, s.clock().Add(-retention))
	if err != nil {
		return 0, err
	}
	for i, item := range items {
		if err := s.purge(ctx, item.UserID, item.ID); err != nil {
			return i, err
		}
	}
	return len(items), nil
}

func (s *Service) purge(ctx context.Context, userID int64, id string) error {
	if err := s.trash.DeleteTrashItem(ctx, userID, id); err != nil {
		return err
	}
	defer s.usage.invalidate(userID)
	if err := s.volume.root.RemoveAll(trashPath(userID, id)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove trash item: %w", err)
	}
	return nil
}
