//go:build !windows

package remote

import (
	"errors"
	"os"
)

func validateSessionFile(info os.FileInfo) error {
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return errors.New("session file must be a regular file with mode 0600")
	}
	return nil
}

func protectSession(data []byte) ([]byte, error) {
	return data, nil
}

func unprotectSession(data []byte) ([]byte, error) {
	return data, nil
}
