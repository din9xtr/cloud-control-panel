package httpapi_test

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/din9xtr/cloud-control-panel/internal/command"
	"github.com/din9xtr/cloud-control-panel/internal/mcpserver"
	"github.com/din9xtr/cloud-control-panel/internal/remote"
)

func TestCLIFullManagement(t *testing.T) {
	c := newEmptyClient(t)
	dir := t.TempDir()
	session := filepath.Join(dir, "session.json")
	run := func(name string, body map[string]any) map[string]any {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		var out, diagnostic bytes.Buffer
		args := []string{"--server", c.base, "--session", session, name, "--data-file", "-"}
		if err := command.Run(t.Context(), "cli", args, bytes.NewReader(data), &out, &diagnostic); err != nil {
			t.Fatalf("%s: %v (%s)", name, err, diagnostic.String())
		}
		if strings.Contains(out.String(), "cloud_session") || strings.Contains(out.String(), adminPassword) {
			t.Fatal("credentials leaked to stdout")
		}
		var result struct {
			Status  int
			Data    map[string]any
			Headers map[string]string
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatalf("%s: %s: %v", name, out.String(), err)
		}
		if result.Status < 200 || result.Status >= 300 {
			t.Fatalf("%s: %+v", name, result)
		}
		if result.Headers != nil {
			result.Data = map[string]any{"headers": result.Headers}
		}
		return result.Data
	}
	empty := map[string]any{}
	if run("setupStatus", empty)["required"] != true {
		t.Fatal("expected first-run setup")
	}
	run("setup", map[string]any{"username": "owner", "password": adminPassword})
	info, err := os.Stat(session)
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("session permissions: %v, %v", info, err)
	}
	run("logout", empty)
	run("login", map[string]any{"username": "owner", "password": adminPassword})
	run("whoami", empty)
	run("mkdir", map[string]any{"path": "garden"})
	run("mkdir", map[string]any{"path": "archive"})
	source := filepath.Join(dir, "seed-catalog-2026.txt")
	payload := bytes.Repeat([]byte("neutral garden fixture\n"), 250000)
	if err := os.WriteFile(source, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	run("upload", map[string]any{"input": source, "folder": "garden"})
	zero := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(zero, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	run("upload", map[string]any{"input": zero, "folder": "garden"})
	folder := run("ls", map[string]any{"path": "garden"})
	if len(folder["entries"].([]any)) != 2 {
		t.Fatalf("folder: %+v", folder)
	}
	run("search", map[string]any{"q": "seed", "path": "garden"})
	destination := filepath.Join(dir, "download.txt")
	run("download", map[string]any{"path": "garden/seed-catalog-2026.txt", "output": destination})
	got, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatal("download differs from uploaded source")
	}
	var stream, streamDiagnostic bytes.Buffer
	if err := command.Run(t.Context(), "cli", []string{"--server", c.base, "--session", session, "download", "--path", "garden/seed-catalog-2026.txt", "--output", "-"}, strings.NewReader(""), &stream, &streamDiagnostic); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stream.Bytes(), payload) || !strings.Contains(streamDiagnostic.String(), `"status": 200`) {
		t.Fatal("binary stdout mixed with result metadata")
	}
	archive := filepath.Join(dir, "garden.zip")
	run("downloadArchive", map[string]any{"path": []string{"garden"}, "output": archive})
	zr, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 2 {
		t.Fatalf("archive entries: %d", len(zr.File))
	}
	_ = zr.Close()
	link := run("createLink", map[string]any{"path": "garden/seed-catalog-2026.txt", "ttlSeconds": 300})
	run("listLinks", empty)
	run("revokeLink", map[string]any{"id": link["id"]})
	run("rename", map[string]any{"path": "garden/seed-catalog-2026.txt", "name": "catalog.txt"})
	run("mv", map[string]any{"paths": []string{"garden/catalog.txt"}, "destination": "archive"})
	run("rm", map[string]any{"paths": []string{"archive/catalog.txt"}})
	trash := run("listTrash", empty)["items"].([]any)
	run("restoreTrashItem", map[string]any{"id": trash[0].(map[string]any)["id"]})
	run("rm", map[string]any{"paths": []string{"archive/catalog.txt"}})
	trash = run("listTrash", empty)["items"].([]any)
	run("deleteTrashItem", map[string]any{"id": trash[0].(map[string]any)["id"]})
	run("emptyTrash", empty)
	run("storageUsage", empty)
	run("getSettings", empty)
	run("updateSettings", map[string]any{"quotaBytes": 0, "trashRetentionDays": 7, "defaultLinkTtlSeconds": 3600})
	run("changeCredentials", map[string]any{"currentPassword": adminPassword, "username": "gardener"})
	run("tusOptions", empty)
	created := run("tusCreate", map[string]any{"Upload-Length": 10, "Upload-Metadata": tusMeta("filename", "notes.txt", "folder", "garden")})
	headers := created["headers"].(map[string]string)
	id := strings.TrimPrefix(headers["Location"], "/api/v1/uploads/")
	run("tusStatus", map[string]any{"id": id})
	run("tusWrite", map[string]any{"id": id, "Upload-Offset": 0, "data": base64.StdEncoding.EncodeToString([]byte("seeds"))})
	resumeSource := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(resumeSource, []byte("seeds grow"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("upload", map[string]any{"input": resumeSource, "uploadId": id})
	created = run("tusCreate", map[string]any{"Upload-Length": 10, "Upload-Metadata": tusMeta("filename", "discard.txt")})
	id = strings.TrimPrefix(created["headers"].(map[string]string)["Location"], "/api/v1/uploads/")
	run("tusTerminate", map[string]any{"id": id})
	sessions := run("listSessions", empty)["items"].([]any)
	for _, raw := range sessions {
		item := raw.(map[string]any)
		if item["current"] == true {
			run("revokeSession", map[string]any{"id": item["id"]})
		}
	}
	var out bytes.Buffer
	err = command.Run(t.Context(), "cli", []string{"--server", c.base, "--session", session, "me"}, strings.NewReader(""), &out, io.Discard)
	var apiErr *remote.APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("revoked session: %v", err)
	}
}

func TestMCPManagementAndTransferBoundary(t *testing.T) {
	c := newEmptyClient(t)
	client, err := remote.NewClient(c.base, "", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	ops, err := remote.Catalog()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	executor := &remote.Executor{Client: client, Root: root, Restricted: true}
	session := connectMCP(t, mcpserver.New(executor, ops, false))
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != len(ops) {
		t.Fatalf("tool coverage: %d, want %d", len(tools.Tools), len(ops))
	}
	call := func(name string, args map[string]any, wantError bool) *mcp.CallToolResult {
		t.Helper()
		result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if result.IsError != wantError {
			t.Fatalf("%s: %+v", name, result)
		}
		return result
	}
	call("setup", map[string]any{"username": "owner", "password": adminPassword}, false)
	call("createFolder", map[string]any{"path": "garden"}, false)
	if err := os.WriteFile(filepath.Join(dir, "catalog.txt"), []byte("garden notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	call("uploadFile", map[string]any{"input": "catalog.txt", "folder": "garden"}, false)
	call("listFolder", map[string]any{"path": "garden"}, false)
	call("downloadFile", map[string]any{"path": "garden/catalog.txt", "output": "copy.txt"}, false)
	got, err := os.ReadFile(filepath.Join(dir, "copy.txt"))
	if err != nil || string(got) != "garden notes" {
		t.Fatal("MCP download mismatch")
	}
	call("downloadFile", map[string]any{"path": "garden/catalog.txt", "output": "copy.txt"}, true)
	call("downloadFile", map[string]any{"path": "garden/catalog.txt", "output": "../escaped.txt"}, true)
	call("uploadFile", map[string]any{"input": "../escaped.txt"}, true)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("private fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "outside")); err != nil {
		t.Fatal(err)
	}
	call("uploadFile", map[string]any{"input": "outside/secret.txt"}, true)
	call("downloadFile", map[string]any{"path": "garden/catalog.txt", "output": "outside/copy.txt"}, true)
	call("createFolder", map[string]any{"path": "../escape"}, true)
	call("createFolder", map[string]any{"path": "garden", "unknown": true}, true)
	call("createFolder", map[string]any{}, true)
	call("logout", map[string]any{}, false)
	call("listFolder", map[string]any{}, true)
	readSession := connectMCP(t, mcpserver.New(executor, ops, true))
	listed, err := readSession.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range listed.Tools {
		if tool.Name == "createFolder" || tool.Name == "login" || tool.Name == "uploadFile" {
			t.Fatalf("write tool exposed in read-only mode: %s", tool.Name)
		}
	}
	noTransfers := connectMCP(t, mcpserver.New(&remote.Executor{Client: client, Restricted: true}, ops, false))
	result, err := noTransfers.CallTool(t.Context(), &mcp.CallToolParams{Name: "uploadFile", Arguments: map[string]any{"input": "catalog.txt"}})
	if err != nil || !result.IsError {
		t.Fatalf("transfer without root: %v, %v", result, err)
	}
}

func connectMCP(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "integration-test", Version: "1.0.0"}, nil)
	session, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}
