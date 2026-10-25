package files

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
)

func (s *Service) StatShared(_ context.Context, userID int64, root, relative Path) (Entry, error) {
	dir, err := s.volume.root.OpenRoot(userPath(userID, root))
	if err != nil {
		return Entry{}, mapFSError(err)
	}
	defer dir.Close()
	info, err := dir.Stat(relative.fsPath())
	if err != nil {
		return Entry{}, mapFSError(err)
	}
	return newEntry(relative, info), nil
}

func (s *Service) OpenShared(_ context.Context, userID int64, root, relative Path) (*os.File, Entry, error) {
	dir, err := s.volume.root.OpenRoot(userPath(userID, root))
	if err != nil {
		return nil, Entry{}, mapFSError(err)
	}
	defer dir.Close()
	f, err := dir.Open(relative.fsPath())
	if err != nil {
		return nil, Entry{}, mapFSError(err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, Entry{}, mapFSError(err)
	}
	if info.IsDir() {
		_ = f.Close()
		return nil, Entry{}, ErrIsDirectory
	}
	return f, newEntry(relative, info), nil
}

func (s *Service) ListShared(_ context.Context, userID int64, root, relative Path) ([]Entry, error) {
	dir, err := s.volume.root.OpenRoot(userPath(userID, root))
	if err != nil {
		return nil, mapFSError(err)
	}
	defer dir.Close()
	f, err := dir.Open(relative.fsPath())
	if err != nil {
		return nil, mapFSError(err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, mapFSError(err)
	}
	if !info.IsDir() {
		return nil, ErrNotDirectory
	}
	children, err := f.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("read shared folder: %w", err)
	}
	entries := make([]Entry, 0, len(children))
	for _, child := range children {
		p, ok := s.visibleChild(relative, child)
		if !ok {
			continue
		}
		info, err := child.Info()
		if err != nil {
			continue
		}
		entries = append(entries, newEntry(p, info))
	}
	sortEntries(entries)
	return entries, nil
}

func (s *Service) WriteSharedArchive(ctx context.Context, userID int64, root Path, paths []Path, w io.Writer) error {
	dir, err := s.volume.root.OpenRoot(userPath(userID, root))
	if err != nil {
		return mapFSError(err)
	}
	defer dir.Close()
	fsys := dir.FS()
	sources := topLevel(paths)
	for _, p := range sources {
		if _, err := fs.Stat(fsys, p.fsPath()); err != nil {
			return mapFSError(err)
		}
	}
	zw := zip.NewWriter(w)
	for _, p := range sources {
		err := fs.WalkDir(fsys, p.fsPath(), func(name string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if d.IsDir() || !d.Type().IsRegular() {
				return nil
			}
			archiveName := relativeTo(p.Parent().fsPath(), name)
			if p.IsRoot() {
				archiveName = root.Name() + "/" + name
			}
			return addToArchive(zw, fsys, name, archiveName, d)
		})
		if err != nil {
			return fmt.Errorf("shared archive: %w", err)
		}
	}
	return zw.Close()
}
