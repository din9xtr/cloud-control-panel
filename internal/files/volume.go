package files

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strconv"
)

const (
	usersDir   = "users"
	uploadsDir = ".system/uploads"
	trashDir   = ".system/trash"
	legacyTus  = ".tus"
	dirPerm    = 0o750
	filePerm   = 0o640
)

type Volume struct {
	dir  string
	root *os.Root
}

func OpenVolume(dir string) (*Volume, error) {
	for _, sub := range []string{usersDir, uploadsDir, trashDir} {
		if err := os.MkdirAll(path.Join(dir, sub), dirPerm); err != nil {
			return nil, fmt.Errorf("prepare volume: %w", err)
		}
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open volume: %w", err)
	}
	return &Volume{dir: dir, root: root}, nil
}

func (v *Volume) Close() error {
	return v.root.Close()
}

func (v *Volume) Dir() string {
	return v.dir
}

func (v *Volume) EnsureUser(userID int64) error {
	if err := v.root.MkdirAll(userDir(userID), dirPerm); err != nil {
		return fmt.Errorf("ensure user dir: %w", err)
	}
	if err := v.root.MkdirAll(stagingDir(userID), dirPerm); err != nil {
		return fmt.Errorf("ensure staging dir: %w", err)
	}
	if err := v.root.MkdirAll(userTrashDir(userID), dirPerm); err != nil {
		return fmt.Errorf("ensure trash dir: %w", err)
	}
	return nil
}

func (v *Volume) CreateStaging(userID int64, id string) (*os.File, error) {
	f, err := v.root.OpenFile(stagingFile(userID, id), os.O_CREATE|os.O_EXCL|os.O_WRONLY, filePerm)
	return f, mapFSError(err)
}

func (v *Volume) OpenStaging(userID int64, id string) (*os.File, error) {
	f, err := v.root.OpenFile(stagingFile(userID, id), os.O_WRONLY, filePerm)
	return f, mapFSError(err)
}

func (v *Volume) RemoveStaging(userID int64, id string) error {
	err := v.root.Remove(stagingFile(userID, id))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return mapFSError(err)
}

func (v *Volume) userFS(userID int64) (fs.FS, error) {
	return fs.Sub(v.root.FS(), userDir(userID))
}

func userDir(userID int64) string {
	return path.Join(usersDir, strconv.FormatInt(userID, 10))
}

func userPath(userID int64, p Path) string {
	return path.Join(userDir(userID), p.fsPath())
}

func stagingDir(userID int64) string {
	return path.Join(uploadsDir, strconv.FormatInt(userID, 10))
}

func stagingFile(userID int64, id string) string {
	return path.Join(stagingDir(userID), id+".part")
}

func userTrashDir(userID int64) string {
	return path.Join(trashDir, strconv.FormatInt(userID, 10))
}

func trashPath(userID int64, id string) string {
	return path.Join(userTrashDir(userID), id)
}

func mapFSError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: %w", ErrNotFound, err)
	case errors.Is(err, fs.ErrExist):
		return fmt.Errorf("%w: %w", ErrExists, err)
	default:
		return err
	}
}
