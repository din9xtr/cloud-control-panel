package remote

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFolderUploadRespectsTransferRoot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("unsafe local source reached remote API")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	client, err := NewClient(srv.URL, "", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	inside, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(inside, "shortcut")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(inside)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	op := findOperation(t, "uploadFolder")
	for _, tt := range []struct {
		name, input string
		root        *os.Root
	}{
		{name: "no transfer root", input: "garden"},
		{name: "absolute source", input: outside, root: root},
		{name: "parent traversal", input: "../outside", root: root},
		{name: "symlink escape", input: "shortcut", root: root},
	} {
		t.Run(tt.name, func(t *testing.T) {
			executor := Executor{Client: client, Root: tt.root, Restricted: true}
			if _, err := executor.Execute(t.Context(), op, map[string]any{"input": tt.input}); err == nil {
				t.Fatal("accepted source outside transfer root")
			}
		})
	}
}
