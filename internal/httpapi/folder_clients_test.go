package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/din9xtr/cloud-control-panel/internal/command"
	"github.com/din9xtr/cloud-control-panel/internal/mcpserver"
	"github.com/din9xtr/cloud-control-panel/internal/remote"
)

func TestFolderFeaturesThroughCLIAndMCP(t *testing.T) {
	for _, kind := range []string{"cli", "mcp"} {
		t.Run(kind, func(t *testing.T) {
			c := newEmptyClient(t)
			dir := t.TempDir()
			for _, relative := range []string{"garden/greenhouse", "garden/empty"} {
				if err := os.MkdirAll(filepath.Join(dir, relative), 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "garden", "greenhouse", "seeds.txt"), []byte("seeds"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "incoming.txt"), []byte("catalog"), 0o600); err != nil {
				t.Fatal(err)
			}
			invoke := folderClient(t, c, dir, kind)
			invoke("setup", map[string]any{"username": "admin", "password": adminPassword})
			result := invoke("uploadFolder", map[string]any{"input": "garden"})
			if result["files"] != float64(1) || result["folders"] != float64(3) {
				t.Fatalf("folder upload = %+v", result)
			}
			c.login()
			if err := os.WriteFile(filepath.Join(dir, "garden", "greenhouse", "sprouts.txt"), []byte("green"), 0o600); err != nil {
				t.Fatal(err)
			}
			partial := c.expect(c.do("POST", "/api/v1/uploads", nil, "Tus-Resumable", "1.0.0", "Upload-Length", "5", "Upload-Metadata", tusMeta("filename", "sprouts.txt", "folder", "garden/greenhouse", "conflict", "skip-identical")), 201)
			partialLocation := partial.header.Get("Location")
			c.expect(c.do("PATCH", partialLocation, []byte("gr"), "Tus-Resumable", "1.0.0", "Upload-Offset", "0", "Content-Type", "application/offset+octet-stream"), 204)
			invoke("uploadFolder", map[string]any{"input": "garden"})
			c.expect(c.do("DELETE", partialLocation, nil, "Tus-Resumable", "1.0.0"), 204)
			complete := c.expect(c.do("GET", "/api/v1/files/content?path=garden/greenhouse/sprouts.txt", nil), 200)
			if string(complete.body) != "green" {
				t.Fatalf("folder retry content: %q", complete.body)
			}
			listing := invoke("listFolder", map[string]any{"path": "garden/greenhouse"})
			entries, ok := listing["entries"].([]any)
			if !ok || len(entries) != 2 {
				t.Fatalf("folder retry created duplicates: %+v", listing)
			}
			invoke("listFolder", map[string]any{"path": "garden/empty"})
			read := invoke("createLink", map[string]any{"path": "garden", "mode": "download"})
			drop := invoke("createLink", map[string]any{"path": "garden", "mode": "upload"})
			readToken := strings.TrimPrefix(read["url"].(string), "/share/")
			dropToken := strings.TrimPrefix(drop["url"].(string), "/share/")
			invoke("logout", map[string]any{})
			invoke("publicFolder", map[string]any{"token": readToken, "path": "greenhouse"})
			invoke("publicFolderContent", map[string]any{"token": readToken, "path": "greenhouse/seeds.txt", "output": "copy.txt"})
			got, err := os.ReadFile(filepath.Join(dir, "copy.txt"))
			if err != nil || string(got) != "seeds" {
				t.Fatalf("public download = %q, %v", got, err)
			}
			invoke("publicFolderArchive", map[string]any{"token": readToken, "path": []string{"greenhouse"}, "output": "garden.zip"})
			invoke("uploadFile", map[string]any{"input": "incoming.txt", "token": dropToken})
			info := invoke("publicFolder", map[string]any{"token": dropToken})
			if _, exists := info["folder"]; exists {
				t.Fatal("file request exposed folder contents")
			}
			c.login()
			gotFile := c.expect(c.do("GET", "/api/v1/files/content?path=garden/incoming.txt", nil), 200)
			if string(gotFile.body) != "catalog" {
				t.Fatalf("public upload = %q", gotFile.body)
			}
		})
	}
}

func folderClient(t *testing.T, c *client, dir, kind string) func(string, map[string]any) map[string]any {
	t.Helper()
	var invoke func(string, map[string]any) []byte
	if kind == "cli" {
		session := filepath.Join(t.TempDir(), "session.json")
		invoke = func(name string, args map[string]any) []byte {
			t.Helper()
			body, err := json.Marshal(args)
			if err != nil {
				t.Fatal(err)
			}
			var out, diagnostic bytes.Buffer
			if err := command.Run(t.Context(), "cli", []string{"--server", c.base, "--session", session, "--transfer-dir", dir, name, "--data-file", "-"}, bytes.NewReader(body), &out, &diagnostic); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			return out.Bytes()
		}
	} else {
		client, err := remote.NewClient(c.base, "", time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		ops, err := remote.Catalog()
		if err != nil {
			t.Fatal(err)
		}
		root, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { root.Close() })
		executor := &remote.Executor{Client: client, Root: root, Restricted: true, Progress: io.Discard}
		session := connectMCP(t, mcpserver.New(executor, ops, false))
		invoke = func(name string, args map[string]any) []byte {
			t.Helper()
			result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
			if err != nil || result.IsError {
				t.Fatalf("%s: %+v, %v", name, result, err)
			}
			body, err := json.Marshal(result.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			return body
		}
	}
	return func(name string, args map[string]any) map[string]any {
		t.Helper()
		var result struct{ Data map[string]any }
		if err := json.Unmarshal(invoke(name, args), &result); err != nil {
			t.Fatal(err)
		}
		return result.Data
	}
}
