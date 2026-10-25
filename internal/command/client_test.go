package command

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/din9xtr/cloud-control-panel/internal/remote"
)

func TestHelpDoesNotOpenServerOrSession(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"help", "ls"}, {"--help"}} {
		var out bytes.Buffer
		if err := Run(t.Context(), "cli", args, strings.NewReader(""), &out, &out); err != nil {
			t.Fatal(err)
		}
		if out.Len() == 0 {
			t.Fatal("empty help")
		}
	}
}

func TestMCPCancellationClosesInput(t *testing.T) {
	input, writer := io.Pipe()
	defer writer.Close()
	observed := &observedInput{reader: input, started: make(chan struct{})}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, "mcp", []string{"--session", filepath.Join(t.TempDir(), "session.json")}, observed, io.Discard, io.Discard)
	}()
	select {
	case <-observed.started:
	case <-time.After(5 * time.Second):
		t.Fatal("MCP did not start reading")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("shutdown error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("MCP failed to stop with an open input stream")
	}
	if _, err := writer.Write([]byte("test")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("MCP input still open: %v", err)
	}
}

type observedInput struct {
	reader  io.ReadCloser
	started chan struct{}
	once    sync.Once
}

func (r *observedInput) Read(p []byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	return r.reader.Read(p)
}

func (r *observedInput) Close() error { return r.reader.Close() }

func TestFlagsAndSecretInput(t *testing.T) {
	ops, err := remote.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	var diagnostic bytes.Buffer
	for _, op := range ops {
		switch op.Name {
		case "login":
			args, err := arguments(op, []string{"--username", "owner"}, strings.NewReader("private-test-password\n"), &diagnostic)
			if err != nil || args["username"] != "owner" || args["password"] != "private-test-password" {
				t.Fatalf("password prompt: %v, %v", args, err)
			}
			if strings.Contains(diagnostic.String(), "private-test-password") {
				t.Fatal("password echoed")
			}
		case "moveEntries":
			args, err := arguments(op, []string{"--paths", `["garden/catalog.txt"]`, "--destination", "archive"}, strings.NewReader(""), &diagnostic)
			if err != nil {
				t.Fatal(err)
			}
			if err := op.Validate(args); err != nil {
				t.Fatal(err)
			}
		case "updateSettings":
			args, err := arguments(op, []string{"--quotaBytes", "9007199254740993", "--trashRetentionDays", "7", "--defaultLinkTtlSeconds", "3600"}, strings.NewReader(""), &diagnostic)
			if err != nil || args["quotaBytes"] != int64(9007199254740993) {
				t.Fatalf("integer precision lost: %v, %v", args, err)
			}
		}
	}
}
