package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func findOperation(t *testing.T, name string) Operation {
	t.Helper()
	ops, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range ops {
		if op.Name == name {
			return op
		}
	}
	t.Fatalf("operation %s not found", name)
	return Operation{}
}

func TestArgumentValidationAndPrecision(t *testing.T) {
	for _, tt := range []struct {
		name, operation, input string
		valid                  bool
	}{
		{"large integer", "updateSettings", `{"quotaBytes":9007199254740993,"trashRetentionDays":7,"defaultLinkTtlSeconds":3600}`, true},
		{"missing query", "searchFiles", `{}`, false},
		{"array query", "downloadArchive", `{"path":["garden","archive"],"output":"garden.zip"}`, true},
		{"empty array", "downloadArchive", `{"path":[],"output":"garden.zip"}`, false},
		{"wrong type", "createFolder", `{"path":123}`, false},
		{"unknown field", "createFolder", `{"path":"garden","extra":true}`, false},
		{"header integer", "tusWrite", `{"id":"test","Upload-Offset":1000000000,"data":""}`, true},
		{"ID traversal", "revokeSession", `{"id":"../account"}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args, err := DecodeArguments([]byte(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			if err := findOperation(t, tt.operation).Validate(args); (err == nil) != tt.valid {
				t.Fatalf("validation: %v, want valid=%v", err, tt.valid)
			}
			if tt.name == "large integer" && args["quotaBytes"] != int64(9007199254740993) {
				t.Fatalf("integer lost precision: %v", args)
			}
		})
	}
	for _, input := range []string{"null", "[]", "{} {}", `{"n":999999999999999999999}`, `{"n":1e999999}`} {
		if _, err := DecodeArguments([]byte(input)); err == nil {
			t.Errorf("accepted invalid arguments %s", input)
		}
	}
}

func TestClientSessionIsolationAndRedirection(t *testing.T) {
	var forwarded bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { forwarded = true; w.WriteHeader(200) }))
	defer target.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "cloud_session", Value: "private-test-token", Path: "/", Expires: time.Now().Add(time.Hour)})
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"user":{"username":"owner"}}`)
			return
		}
		if cookie, err := r.Cookie("cloud_session"); err != nil || cookie.Value != "private-test-token" {
			t.Error("session not restored")
		}
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer srv.Close()
	state := filepath.Join(t.TempDir(), "session.json")
	client, err := NewClient(srv.URL, state, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Call(t.Context(), findOperation(t, "login"), map[string]any{"username": "owner", "password": "private-test-password"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "private-test-token") || strings.Contains(string(encoded), "Set-Cookie") {
		t.Fatal("token leaked")
	}
	client, err = NewClient(srv.URL, state, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Call(t.Context(), findOperation(t, "me"), map[string]any{}, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusFound || forwarded {
		t.Fatalf("redirect followed: %v, %v", forwarded, err)
	}
	if _, err := NewClient(target.URL, state, time.Second); err == nil {
		t.Fatal("cross-server session accepted")
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(state, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := NewClient(srv.URL, state, time.Second); err == nil {
			t.Fatal("world-readable session accepted")
		}
	}
}

func TestDownloadFailurePreservesExistingFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = io.WriteString(w, "short")
	}))
	defer srv.Close()
	client, err := NewClient(srv.URL, "", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	executor := &Executor{Client: client, Root: root, Restricted: true}
	op := findOperation(t, "downloadFile")
	if _, err := executor.Execute(t.Context(), op, map[string]any{"path": "garden/catalog.txt", "output": "partial.txt"}); err == nil {
		t.Fatal("incomplete download accepted")
	}
	if _, err := root.Stat("partial.txt"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial file not removed")
	}
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := executor.Execute(t.Context(), op, map[string]any{"path": "garden/catalog.txt", "output": "existing.txt"}); err == nil {
		t.Fatal("existing file overwritten")
	}
	got, err := os.ReadFile(filepath.Join(dir, "existing.txt"))
	if err != nil || string(got) != "keep" {
		t.Fatal("existing destination changed")
	}
}

func TestUploadResumesAfterAmbiguousFailure(t *testing.T) {
	payload := []byte("garden notes for spring")
	var stored bytes.Buffer
	failed := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Location", "/api/v1/uploads/test-upload")
			w.WriteHeader(http.StatusCreated)
		case http.MethodHead:
			w.Header().Set("Upload-Offset", fmt.Sprint(stored.Len()))
			w.Header().Set("Upload-Length", fmt.Sprint(len(payload)))
		case http.MethodPatch:
			if r.Header.Get("Upload-Offset") != fmt.Sprint(stored.Len()) {
				t.Error("wrong resume offset")
			}
			if !failed {
				_, _ = io.CopyN(&stored, r.Body, 7)
				failed = true
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, _ = io.Copy(&stored, r.Body)
			w.Header().Set("Upload-Offset", fmt.Sprint(stored.Len()))
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	client, err := NewClient(srv.URL, "", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "catalog.txt")
	if err := os.WriteFile(source, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var progress bytes.Buffer
	if _, err := client.Upload(t.Context(), f, map[string]any{}, &progress); err == nil || !strings.Contains(err.Error(), "test-upload") {
		t.Fatalf("missing resume ID: %v", err)
	}
	if _, err := client.Upload(t.Context(), f, map[string]any{"uploadId": "test-upload"}, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored.Bytes(), payload) {
		t.Fatalf("resume duplicated or lost bytes: %q", stored.Bytes())
	}
}

func TestCanceledRequestAndInvalidOrigin(t *testing.T) {
	for _, origin := range []string{"file:///tmp", "https://user:password@example.com", "https://example.com/api", "https://example.com?token=x"} {
		if _, err := NewClient(origin, "", time.Second); err == nil {
			t.Fatalf("accepted origin %s", origin)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	client, err := NewClient("http://127.0.0.1:1", "", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Call(ctx, findOperation(t, "me"), map[string]any{}, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestTusHeadersFollowContractRatherThanOperationName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Tus-Resumable") != "1.0.0" {
			t.Error("missing tus version header")
		}
		w.Header().Set("Upload-Offset", "0")
		w.Header().Set("Upload-Length", "10")
	}))
	defer srv.Close()
	client, err := NewClient(srv.URL, "", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	op := findOperation(t, "tusStatus")
	op.Name = "anotherUploadStatus"
	if _, err := client.Call(t.Context(), op, map[string]any{"id": "test-upload"}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadPublishesOnlyAfterCompleteTransfer(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "6")
		_, _ = io.WriteString(w, "abc")
		w.(http.Flusher).Flush()
		close(started)
		<-release
		_, _ = io.WriteString(w, "def")
	}))
	defer srv.Close()
	client, err := NewClient(srv.URL, "", time.Second*5)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	executor := &Executor{Client: client, Root: root, Restricted: true}
	op := findOperation(t, "downloadFile")
	done := make(chan error, 1)
	go func() {
		_, err := executor.Execute(t.Context(), op, map[string]any{"path": "garden/catalog.txt", "output": "catalog.txt"})
		done <- err
	}()
	<-started
	_, statErr := root.Stat("catalog.txt")
	close(release)
	transferErr := <-done
	if !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("partial destination visible: %v", statErr)
	}
	if transferErr != nil {
		t.Fatal(transferErr)
	}
	data, err := os.ReadFile(filepath.Join(dir, "catalog.txt"))
	if err != nil || string(data) != "abcdef" {
		t.Fatalf("download = %q, %v", data, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary file leaked: %v, %v", entries, err)
	}
}
