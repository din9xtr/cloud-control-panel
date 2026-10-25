package files

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"path"
)

func (s *Service) WriteArchive(ctx context.Context, userID int64, paths []Path, w io.Writer) error {
	fsys, err := s.volume.userFS(userID)
	if err != nil {
		return err
	}
	sources := topLevel(paths)
	for _, p := range sources {
		if p.IsRoot() || isReserved(p) {
			return ErrInvalidPath
		}
		if _, err := fs.Stat(fsys, p.fsPath()); err != nil {
			return mapFSError(err)
		}
	}
	zw := zip.NewWriter(w)
	for _, p := range sources {
		base := p.Parent().fsPath()
		err := fs.WalkDir(fsys, p.fsPath(), func(name string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if d.IsDir() || !d.Type().IsRegular() {
				return nil
			}
			return addToArchive(zw, fsys, name, relativeTo(base, name), d)
		})
		if err != nil {
			return fmt.Errorf("archive entry: %w", err)
		}
	}
	return zw.Close()
}

func addToArchive(zw *zip.Writer, fsys fs.FS, name, archiveName string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = archiveName
	header.Method = zip.Deflate
	switch CategoryOf(d.Name()) {
	case CategoryImage, CategoryVideo, CategoryAudio, CategoryArchive:
		header.Method = zip.Store
	}
	dst, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	src, err := fsys.Open(name)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(dst, src)
	return err
}

func relativeTo(base, name string) string {
	if base == "." {
		return name
	}
	rel := name[len(base):]
	return path.Clean("./" + rel)
}

func ArchiveName(paths []Path) string {
	if len(paths) == 1 {
		return paths[0].Name() + ".zip"
	}
	return "files.zip"
}
