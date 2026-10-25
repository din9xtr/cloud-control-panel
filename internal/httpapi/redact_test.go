package httpapi

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestRedactRemovesPathsFromErrors(t *testing.T) {
	secret := "users/1/garden/seed-catalog-2026.pdf"
	tests := []struct {
		name string
		err  error
	}{
		{"path error", fmt.Errorf("open: %w", &fs.PathError{Op: "open", Path: secret, Err: fs.ErrPermission})},
		{"link error", fmt.Errorf("move: %w", &os.LinkError{Op: "rename", Old: secret, New: "users/1/x", Err: fs.ErrExist})},
		{"joined", errors.Join(errors.New("first"), &fs.PathError{Op: "stat", Path: secret, Err: fs.ErrNotExist})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redact(tt.err)
			if strings.Contains(got, "seed-catalog") || !strings.Contains(got, redactedPath) {
				t.Fatalf("redact() = %q", got)
			}
		})
	}
}
