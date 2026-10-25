package files

import "errors"

var (
	ErrInvalidPath   = errors.New("invalid path")
	ErrInvalidName   = errors.New("invalid name")
	ErrNotFound      = errors.New("not found")
	ErrExists        = errors.New("already exists")
	ErrNotDirectory  = errors.New("not a directory")
	ErrIsDirectory   = errors.New("is a directory")
	ErrMoveIntoSelf  = errors.New("cannot move a folder into itself")
	ErrRootOperation = errors.New("operation not allowed on root")
	ErrQuotaExceeded = errors.New("storage quota exceeded")
	ErrDiskFull      = errors.New("not enough disk space")
)
