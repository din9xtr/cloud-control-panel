package remote

import (
	"bytes"
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func validateSessionFile(info os.FileInfo) error {
	if !info.Mode().IsRegular() {
		return errors.New("session file must be a regular file")
	}
	return nil
}

func protectSession(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty session")
	}
	input := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var output windows.DataBlob
	if err := windows.CryptProtectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data))) }()
	return bytes.Clone(unsafe.Slice(output.Data, output.Size)), nil
}

func unprotectSession(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty session")
	}
	input := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var output windows.DataBlob
	if err := windows.CryptUnprotectData(&input, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &output); err != nil {
		return nil, err
	}
	defer func() { _, _ = windows.LocalFree(windows.Handle(unsafe.Pointer(output.Data))) }()
	return bytes.Clone(unsafe.Slice(output.Data, output.Size)), nil
}
