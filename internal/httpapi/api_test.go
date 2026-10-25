package httpapi_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"golang.org/x/crypto/bcrypt"

	"github.com/din9xtr/cloud-control-panel/internal/app"
	"github.com/din9xtr/cloud-control-panel/internal/config"
	"github.com/din9xtr/cloud-control-panel/internal/httpapi"
)

const adminPassword = "integration-pass"

type client struct {
	t        *testing.T
	base     string
	http     *http.Client
	contract validator.Validator
}

type response struct {
	status int
	header http.Header
	body   []byte
}

func (r response) json(t *testing.T, dst any) {
	t.Helper()
	if err := json.Unmarshal(r.body, dst); err != nil {
		t.Fatalf("decode %s: %v", r.body, err)
	}
}

func newClient(t *testing.T) *client {
	t.Helper()
	c := newEmptyClient(t)
	setup := &client{t: t, base: c.base, http: &http.Client{}, contract: c.contract}
	setup.expect(setup.do("POST", "/api/v1/setup", map[string]string{"username": "admin", "password": adminPassword}), 201)
	return c
}

func newEmptyClient(t *testing.T) *client {
	t.Helper()
	dir := t.TempDir()
	cfg := config.Config{
		DBPath:        filepath.Join(dir, "cloud.db"),
		StorageDir:    filepath.Join(dir, "storage"),
		SecureCookies: httpapi.CookiesNeverSecure,
		PasswordCost:  bcrypt.MinCost,
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	a, err := app.New(context.Background(), cfg, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	if err := a.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	handler, err := a.Handler()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	spec, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := libopenapi.NewDocument(spec)
	if err != nil {
		t.Fatal(err)
	}
	v, errs := validator.NewValidator(doc)
	if len(errs) > 0 {
		t.Fatalf("load contract: %v", errs)
	}
	if ok, verrs := v.ValidateDocument(); !ok {
		t.Fatalf("invalid contract: %v", verrs)
	}
	jar, _ := cookiejar.New(nil)
	return &client{t: t, base: srv.URL, http: &http.Client{Jar: jar}, contract: v}
}

func (c *client) do(method, path string, body any, headers ...string) response {
	c.t.Helper()
	var reader io.Reader
	contentType := ""
	switch b := body.(type) {
	case nil:
	case []byte:
		reader = bytes.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			c.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
		contentType = "application/json"
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		c.t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	if strings.HasPrefix(path, "/api/v1/") && req.Header.Get("X-Skip-Contract") == "" {
		c.checkContract(req, resp, raw)
	}
	return response{status: resp.StatusCode, header: resp.Header, body: raw}
}

func (c *client) checkContract(req *http.Request, resp *http.Response, raw []byte) {
	c.t.Helper()
	contractReq := req.Clone(context.Background())
	contractReq.URL.Path = strings.TrimPrefix(req.URL.Path, "/api/v1")
	contractReq.Body = io.NopCloser(bytes.NewReader(nil))
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	if ok, errs := c.contract.ValidateHttpResponse(contractReq, resp); !ok {
		for _, e := range errs {
			c.t.Errorf("contract violation %s %s -> %d: %s %s", req.Method, req.URL.Path, resp.StatusCode, e.Message, e.Reason)
		}
	}
}

func (c *client) expect(r response, status int) response {
	c.t.Helper()
	if r.status != status {
		c.t.Fatalf("status = %d, want %d: %s", r.status, status, r.body)
	}
	return r
}

func (c *client) login() {
	c.t.Helper()
	c.expect(c.do("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": adminPassword}), 200)
}

func tusMeta(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, pairs[i]+" "+base64.StdEncoding.EncodeToString([]byte(pairs[i+1])))
	}
	return strings.Join(parts, ",")
}

func TestAuthFlow(t *testing.T) {
	c := newClient(t)
	c.expect(c.do("GET", "/api/v1/files", nil), 401)
	c.expect(c.do("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": "nope"}), 401)

	r := c.expect(c.do("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": adminPassword}), 200)
	cookie := r.header.Get("Set-Cookie")
	for _, attr := range []string{"cloud_session=", "HttpOnly", "SameSite=Strict"} {
		if !strings.Contains(cookie, attr) {
			t.Fatalf("cookie %q misses %s", cookie, attr)
		}
	}
	var me struct{ User struct{ Username string } }
	c.expect(c.do("GET", "/api/v1/auth/me", nil), 200).json(t, &me)
	if me.User.Username != "admin" {
		t.Fatalf("me = %+v", me)
	}

	var sessions struct{ Items []struct{ Current bool } }
	c.expect(c.do("GET", "/api/v1/account/sessions", nil), 200).json(t, &sessions)
	if len(sessions.Items) != 2 || sessions.Items[0].Current == sessions.Items[1].Current {
		t.Fatalf("sessions = %+v", sessions)
	}
	c.expect(c.do("PUT", "/api/v1/account/credentials", map[string]string{"currentPassword": "wrong", "newPassword": "another-pass"}), 403)
	c.expect(c.do("PUT", "/api/v1/account/credentials", map[string]string{"currentPassword": adminPassword, "username": "owner", "newPassword": "another-pass"}), 200)
	c.expect(c.do("POST", "/api/v1/auth/logout", nil), 204)
	c.expect(c.do("GET", "/api/v1/auth/me", nil), 401)
	c.expect(c.do("POST", "/api/v1/auth/login", map[string]string{"username": "owner", "password": "another-pass"}), 200)
}

func TestFirstRunSetup(t *testing.T) {
	c := newEmptyClient(t)
	var status struct{ Required bool }
	c.expect(c.do("GET", "/api/v1/setup", nil), 200).json(t, &status)
	if !status.Required {
		t.Fatal("setup must be required without an account")
	}
	c.expect(c.do("POST", "/api/v1/setup", map[string]string{"username": "owner", "password": "short"}), 400)
	c.expect(c.do("POST", "/api/v1/setup", map[string]string{"username": "owner", "password": "first-run-pass"}), 201)
	c.expect(c.do("GET", "/api/v1/files", nil), 200)
	c.expect(c.do("POST", "/api/v1/setup", map[string]string{"username": "intruder", "password": "first-run-pass"}), 409)
	c.expect(c.do("GET", "/api/v1/setup", nil), 200).json(t, &status)
	if status.Required {
		t.Fatal("setup must be finished")
	}
}

func TestLoginIsThrottled(t *testing.T) {
	c := newClient(t)
	for range 5 {
		c.expect(c.do("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": "wrong"}), 401)
	}
	r := c.expect(c.do("POST", "/api/v1/auth/login", map[string]string{"username": "admin", "password": adminPassword}), 429)
	if r.header.Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
}

func TestFolderManagement(t *testing.T) {
	c := newClient(t)
	c.login()
	c.expect(c.do("POST", "/api/v1/files/folders", map[string]string{"path": "garden"}), 201)
	c.expect(c.do("POST", "/api/v1/files/folders", map[string]string{"path": "garden/2026"}), 201)
	c.expect(c.do("POST", "/api/v1/files/folders", map[string]string{"path": "garden"}), 409)
	c.expect(c.do("POST", "/api/v1/files/folders", map[string]string{"path": "../escape"}), 400)
	c.expect(c.do("GET", "/api/v1/files?path=..%2Fetc", nil), 400)
	c.expect(c.do("POST", "/api/v1/files/rename", map[string]string{"path": "garden/2026", "name": "archive"}), 200)
	c.expect(c.do("POST", "/api/v1/files/folders", map[string]string{"path": "inbox"}), 201)
	c.expect(c.do("POST", "/api/v1/files/move", map[string]any{"paths": []string{"inbox"}, "destination": "garden"}), 204)
	c.expect(c.do("POST", "/api/v1/files/move", map[string]any{"paths": []string{"garden"}, "destination": "garden/inbox"}), 400)

	var folder struct {
		Path    string
		Entries []struct{ Name, Kind string }
	}
	c.expect(c.do("GET", "/api/v1/files?path=garden", nil), 200).json(t, &folder)
	if len(folder.Entries) != 2 || folder.Entries[0].Name != "archive" || folder.Entries[1].Name != "inbox" {
		t.Fatalf("folder = %+v", folder)
	}
	var found struct{ Entries []struct{ Path string } }
	c.expect(c.do("GET", "/api/v1/files/search?q=INB", nil), 200).json(t, &found)
	if len(found.Entries) != 1 || found.Entries[0].Path != "garden/inbox" {
		t.Fatalf("search = %+v", found)
	}
	c.expect(c.do("POST", "/api/v1/files/folders", map[string]any{"path": "x", "extra": 1}), 400)
}

func TestResumableUploadDownloadAndShare(t *testing.T) {
	c := newClient(t)
	c.login()
	c.expect(c.do("POST", "/api/v1/files/folders", map[string]string{"path": "video"}), 201)
	payload := bytes.Repeat([]byte("0123456789"), 10000)

	opts := c.expect(c.do("OPTIONS", "/api/v1/uploads", nil), 204)
	if !strings.Contains(opts.header.Get("Tus-Extension"), "creation") {
		t.Fatalf("Tus-Extension = %q", opts.header.Get("Tus-Extension"))
	}
	c.expect(c.do("POST", "/api/v1/uploads", nil, "Upload-Length", "10", "Upload-Metadata", tusMeta("filename", "a")), 412)
	created := c.expect(c.do("POST", "/api/v1/uploads", nil,
		"Tus-Resumable", "1.0.0", "Upload-Length", "100000", "Upload-Metadata", tusMeta("filename", "clip.mp4", "folder", "video")), 201)
	location := created.header.Get("Location")
	if !strings.HasPrefix(location, "/api/v1/uploads/") {
		t.Fatalf("Location = %q", location)
	}
	patch := func(offset string, chunk []byte) response {
		return c.do("PATCH", location, chunk, "Tus-Resumable", "1.0.0", "Upload-Offset", offset, "Content-Type", "application/offset+octet-stream")
	}
	c.expect(patch("0", payload[:40000]), 204)
	c.expect(patch("0", payload[:10]), 409)
	head := c.expect(c.do("HEAD", location, nil, "Tus-Resumable", "1.0.0"), 200)
	if head.header.Get("Upload-Offset") != "40000" {
		t.Fatalf("Upload-Offset = %q", head.header.Get("Upload-Offset"))
	}
	c.expect(patch("40000", payload[40000:]), 204)
	c.expect(c.do("HEAD", location, nil, "Tus-Resumable", "1.0.0"), 404)

	ranged := c.expect(c.do("GET", "/api/v1/files/content?path=video%2Fclip.mp4", nil, "Range", "bytes=10-19"), 206)
	if string(ranged.body) != "0123456789" {
		t.Fatalf("range body = %q", ranged.body)
	}
	inline := c.expect(c.do("GET", "/api/v1/files/content?path=video%2Fclip.mp4&inline=true", nil), 200)
	if !strings.HasPrefix(inline.header.Get("Content-Disposition"), "inline") || inline.header.Get("Content-Type") != "video/mp4" {
		t.Fatalf("inline headers = %v", inline.header)
	}

	archive := c.expect(c.do("GET", "/api/v1/files/archive?path=video", nil), 200)
	zr, err := zip.NewReader(bytes.NewReader(archive.body), int64(len(archive.body)))
	if err != nil || len(zr.File) != 1 || zr.File[0].Name != "video/clip.mp4" {
		t.Fatalf("archive = %v, %v", zr, err)
	}

	var link struct{ ID, URL string }
	c.expect(c.do("POST", "/api/v1/links", map[string]any{"path": "video/clip.mp4"}), 201).json(t, &link)
	c.expect(c.do("POST", "/api/v1/links", map[string]any{"path": "video/clip.mp4", "mode": "upload"}), 400)
	anonymous := &client{t: t, base: c.base, http: &http.Client{}, contract: c.contract}
	shared := anonymous.expect(anonymous.do("GET", link.URL, nil), 200)
	if !bytes.Equal(shared.body, payload) || !strings.HasPrefix(shared.header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("shared download mismatch: %v", shared.header)
	}
	var folder struct {
		Entries []struct{ Link *struct{ ID string } }
	}
	c.expect(c.do("GET", "/api/v1/files?path=video", nil), 200).json(t, &folder)
	if folder.Entries[0].Link == nil || folder.Entries[0].Link.ID != link.ID {
		t.Fatalf("entry link = %+v", folder.Entries)
	}
	var links struct{ Items []struct{ Downloads int } }
	c.expect(c.do("GET", "/api/v1/links", nil), 200).json(t, &links)
	if len(links.Items) != 1 || links.Items[0].Downloads != 1 {
		t.Fatalf("links = %+v", links)
	}
	c.expect(c.do("DELETE", "/api/v1/links/"+link.ID, nil), 204)
	gone := anonymous.expect(anonymous.do("GET", link.URL, nil), 404)
	if !strings.Contains(string(gone.body), "no longer available") {
		t.Fatalf("missing link page = %s", gone.body)
	}
	missing := anonymous.expect(anonymous.do("GET", "/s/short", nil), 404)
	if csp := missing.header.Get("Content-Security-Policy"); strings.Contains(csp, "unsafe-inline") || !strings.Contains(csp, "'sha256-") {
		t.Fatalf("missing link CSP = %q", csp)
	}
}

func TestUploadRespectsQuota(t *testing.T) {
	c := newClient(t)
	c.login()
	c.expect(c.do("PUT", "/api/v1/settings", map[string]any{"quotaBytes": 1000, "trashRetentionDays": 30, "defaultLinkTtlSeconds": 86400}), 200)
	c.expect(c.do("POST", "/api/v1/uploads", nil,
		"Tus-Resumable", "1.0.0", "Upload-Length", "5000", "Upload-Metadata", tusMeta("filename", "big.bin")), 507)
}

func TestTrashLifecycle(t *testing.T) {
	c := newClient(t)
	c.login()
	c.expect(c.do("POST", "/api/v1/files/folders", map[string]string{"path": "old"}), 201)
	c.expect(c.do("POST", "/api/v1/files/trash", map[string]any{"paths": []string{"old"}}), 204)

	var trash struct {
		Items []struct{ ID, OriginalPath string }
	}
	c.expect(c.do("GET", "/api/v1/trash", nil), 200).json(t, &trash)
	if len(trash.Items) != 1 || trash.Items[0].OriginalPath != "old" {
		t.Fatalf("trash = %+v", trash)
	}
	var restored struct{ Path string }
	c.expect(c.do("POST", "/api/v1/trash/"+trash.Items[0].ID+"/restore", nil), 200).json(t, &restored)
	if restored.Path != "old" {
		t.Fatalf("restored = %+v", restored)
	}
	c.expect(c.do("POST", "/api/v1/files/trash", map[string]any{"paths": []string{"old"}}), 204)
	c.expect(c.do("GET", "/api/v1/trash", nil), 200).json(t, &trash)
	c.expect(c.do("DELETE", "/api/v1/trash/"+trash.Items[0].ID, nil), 204)
	c.expect(c.do("DELETE", "/api/v1/trash/"+trash.Items[0].ID, nil), 404)
	c.expect(c.do("DELETE", "/api/v1/trash", nil), 204)
}

func TestStorageAndSettings(t *testing.T) {
	c := newClient(t)
	c.login()
	var storage struct {
		QuotaBytes int64
		Disk       struct{ TotalBytes int64 }
	}
	c.expect(c.do("GET", "/api/v1/storage", nil), 200).json(t, &storage)
	if storage.Disk.TotalBytes == 0 {
		t.Fatalf("storage = %+v", storage)
	}
	c.expect(c.do("GET", "/api/v1/settings", nil), 200)
	c.expect(c.do("PUT", "/api/v1/settings", map[string]any{"quotaBytes": -1, "trashRetentionDays": 30, "defaultLinkTtlSeconds": 86400}), 400)
	c.expect(c.do("PUT", "/api/v1/settings", map[string]any{"quotaBytes": 0, "trashRetentionDays": 106751991167301, "defaultLinkTtlSeconds": 86400}), 400)
	c.expect(c.do("PUT", "/api/v1/settings", map[string]any{"quotaBytes": 0, "trashRetentionDays": 30, "defaultLinkTtlSeconds": 9223372036854}), 400)
	var saved struct{ TrashRetentionDays int }
	c.expect(c.do("PUT", "/api/v1/settings", map[string]any{"quotaBytes": 0, "trashRetentionDays": 7, "defaultLinkTtlSeconds": 3600}), 200).json(t, &saved)
	if saved.TrashRetentionDays != 7 {
		t.Fatalf("settings = %+v", saved)
	}
}

func TestSecurityHeadersAndCrossOrigin(t *testing.T) {
	c := newClient(t)
	c.login()
	r := c.do("GET", "/api/v1/auth/me", nil)
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy"} {
		if r.header.Get(h) == "" {
			t.Errorf("missing %s", h)
		}
	}
	forged := c.do("POST", "/api/v1/files/folders", map[string]string{"path": "evil"}, "Origin", "https://evil.example", "Sec-Fetch-Site", "cross-site", "X-Skip-Contract", "1")
	if forged.status != http.StatusForbidden {
		t.Fatalf("cross-site request status = %d", forged.status)
	}
	plain := c.do("POST", "/api/v1/files/folders", []byte(`{"path":"x"}`), "Content-Type", "text/plain", "X-Skip-Contract", "1")
	if plain.status != http.StatusUnsupportedMediaType {
		t.Fatalf("non-JSON body status = %d", plain.status)
	}
	c.expect(c.do("GET", "/healthz", nil), 200)
	c.expect(c.do("GET", "/api/v1/nope", nil, "X-Skip-Contract", "1"), 404)
}
