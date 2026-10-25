package httpapi_test

import (
	"archive/zip"
	"bytes"
	"net/http"
	"strings"
	"testing"
)

type testLink struct{ ID, URL, Mode, Kind string }

func (c *client) folderLink(path, mode string) testLink {
	c.t.Helper()
	var link testLink
	c.expect(c.do("POST", "/api/v1/links", map[string]any{"path": path, "mode": mode}), 201).json(c.t, &link)
	if link.Mode != mode || link.Kind != "dir" {
		c.t.Fatalf("link = %+v", link)
	}
	return link
}

func publicAPI(link testLink) string {
	return "/api/v1/public/links/" + strings.TrimPrefix(link.URL, "/share/")
}

func (c *client) putFile(folder, name string, content []byte) {
	c.t.Helper()
	r := c.expect(c.do("POST", "/api/v1/uploads", nil, "Tus-Resumable", "1.0.0", "Upload-Length", "4", "Upload-Metadata", tusMeta("filename", name, "folder", folder)), 201)
	c.expect(c.do("PATCH", r.header.Get("Location"), content, "Tus-Resumable", "1.0.0", "Upload-Offset", "0", "Content-Type", "application/offset+octet-stream"), 204)
}

func TestPublicFolderBrowseDownloadAndRevoke(t *testing.T) {
	c := newClient(t)
	c.login()
	for _, path := range []string{"garden", "garden/greenhouse", "private"} {
		c.expect(c.do("POST", "/api/v1/files/folders", map[string]string{"path": path}), 201)
	}
	c.putFile("garden/greenhouse", "seeds.txt", []byte("seed"))
	c.putFile("private", "notes.txt", []byte("note"))
	link := c.folderLink("garden", "download")
	base := publicAPI(link)
	anonymous := &client{t: t, base: c.base, http: &http.Client{}, contract: c.contract}
	root := anonymous.expect(anonymous.do("GET", base, nil), 200)
	if strings.Contains(string(root.body), "garden/greenhouse") || !strings.Contains(string(root.body), `"path":"greenhouse"`) {
		t.Fatalf("public paths must be relative: %s", root.body)
	}
	anonymous.expect(anonymous.do("GET", base+"?path=greenhouse", nil), 200)
	file := anonymous.expect(anonymous.do("GET", base+"/content?path=greenhouse/seeds.txt", nil), 200)
	if string(file.body) != "seed" || !strings.HasPrefix(file.header.Get("Content-Disposition"), "attachment") {
		t.Fatalf("file = %+v", file)
	}
	ranged := anonymous.expect(anonymous.do("GET", base+"/content?path=greenhouse/seeds.txt", nil, "Range", "bytes=1-2"), 206)
	if string(ranged.body) != "ee" {
		t.Fatalf("range = %q", ranged.body)
	}
	for _, suffix := range []string{"?path=../private", "/content?path=greenhouse/../../private/notes.txt", "/archive?path=../private"} {
		anonymous.expect(anonymous.do("GET", base+suffix, nil), 400)
	}
	for _, suffix := range []string{"", "?path=greenhouse/seeds.txt"} {
		archive := anonymous.expect(anonymous.do("GET", base+"/archive"+suffix, nil), 200)
		zr, err := zip.NewReader(bytes.NewReader(archive.body), int64(len(archive.body)))
		if err != nil || len(zr.File) != 1 {
			t.Fatalf("archive = %v, %v", zr, err)
		}
		if strings.Contains(zr.File[0].Name, "private") || !strings.HasSuffix(zr.File[0].Name, "seeds.txt") {
			t.Fatalf("archive entry = %q", zr.File[0].Name)
		}
	}
	anonymous.expect(anonymous.do("POST", base+"/uploads", nil, "Tus-Resumable", "1.0.0"), 404)
	c.expect(c.do("POST", "/api/v1/files/rename", map[string]string{"path": "garden", "name": "orchard"}), 200)
	anonymous.expect(anonymous.do("GET", base+"/content?path=greenhouse/seeds.txt", nil), 200)
	c.expect(c.do("DELETE", "/api/v1/links/"+link.ID, nil), 204)
	anonymous.expect(anonymous.do("GET", base, nil), 404)
	anonymous.expect(anonymous.do("GET", base+"/content?path=greenhouse/seeds.txt", nil), 404)
}

func TestFileDropIsolationResumeAndRevoke(t *testing.T) {
	c := newClient(t)
	c.login()
	c.expect(c.do("POST", "/api/v1/files/folders", map[string]string{"path": "inbox"}), 201)
	c.putFile("inbox", "seeds.txt", []byte("seed"))
	link := c.folderLink("inbox", "upload")
	other := c.folderLink("inbox", "upload")
	base := publicAPI(link)
	anonymous := &client{t: t, base: c.base, http: &http.Client{}, contract: c.contract}
	info := anonymous.expect(anonymous.do("GET", base, nil), 200)
	if strings.Contains(string(info.body), "seeds.txt") || strings.Contains(string(info.body), `"folder"`) {
		t.Fatalf("upload link leaked contents: %s", info.body)
	}
	for _, suffix := range []string{"/content?path=seeds.txt", "/archive"} {
		anonymous.expect(anonymous.do("GET", base+suffix, nil), 404)
	}
	create := func(folder, size string) response {
		return anonymous.do("POST", base+"/uploads", nil, "Tus-Resumable", "1.0.0", "Upload-Length", size, "Upload-Metadata", tusMeta("filename", "seeds.txt", "folder", folder))
	}
	anonymous.expect(anonymous.do("POST", base+"/uploads", nil, "Tus-Resumable", "1.0.0", "Upload-Length", "4", "Upload-Metadata", tusMeta("filename", "seeds.txt", "conflict", "skip-identical")), 400)
	anonymous.expect(create("elsewhere", "4"), 400)
	anonymous.expect(create("", "5368709121"), 400)
	u := anonymous.expect(create("", "4"), 201)
	location := u.header.Get("Location")
	id := location[strings.LastIndex(location, "/")+1:]
	anonymous.expect(anonymous.do("HEAD", publicAPI(other)+"/uploads/"+id, nil, "Tus-Resumable", "1.0.0"), 404)
	patch := func(offset, body string) response {
		return anonymous.do("PATCH", location, []byte(body), "Tus-Resumable", "1.0.0", "Upload-Offset", offset, "Content-Type", "application/offset+octet-stream")
	}
	anonymous.expect(patch("0", "ne"), 204)
	head := anonymous.expect(anonymous.do("HEAD", location, nil, "Tus-Resumable", "1.0.0"), 200)
	if head.header.Get("Upload-Offset") != "2" {
		t.Fatalf("offset = %s", head.header.Get("Upload-Offset"))
	}
	anonymous.expect(patch("2", "ws"), 204)
	if got := c.expect(c.do("GET", "/api/v1/files/content?path=inbox/seeds.txt", nil), 200); string(got.body) != "seed" {
		t.Fatal("existing file overwritten")
	}
	if got := c.expect(c.do("GET", "/api/v1/files/content?path=inbox/seeds%20(1).txt", nil), 200); string(got.body) != "news" {
		t.Fatal("uploaded file missing")
	}
	u = anonymous.expect(create("", "4"), 201)
	location = u.header.Get("Location")
	c.expect(c.do("DELETE", "/api/v1/links/"+link.ID, nil), 204)
	anonymous.expect(patch("0", "test"), 404)
	anonymous.expect(anonymous.do("HEAD", location, nil, "Tus-Resumable", "1.0.0"), 404)
}

func TestFileDropCannotAccessPrivateUploadAndRespectsQuota(t *testing.T) {
	c := newClient(t)
	c.login()
	c.expect(c.do("POST", "/api/v1/files/folders", map[string]string{"path": "inbox"}), 201)
	link := c.folderLink("inbox", "upload")
	private := c.expect(c.do("POST", "/api/v1/uploads", nil, "Tus-Resumable", "1.0.0", "Upload-Length", "4", "Upload-Metadata", tusMeta("filename", "seed.txt", "folder", "inbox")), 201)
	id := strings.TrimPrefix(private.header.Get("Location"), "/api/v1/uploads/")
	anonymous := &client{t: t, base: c.base, http: &http.Client{}, contract: c.contract}
	for _, method := range []string{"HEAD", "PATCH", "DELETE"} {
		anonymous.expect(anonymous.do(method, publicAPI(link)+"/uploads/"+id, nil, "Tus-Resumable", "1.0.0"), 404)
	}
	c.expect(c.do("PUT", "/api/v1/settings", map[string]any{"quotaBytes": 5, "trashRetentionDays": 30, "defaultLinkTtlSeconds": 86400}), 200)
	anonymous.expect(anonymous.do("POST", publicAPI(link)+"/uploads", nil, "Tus-Resumable", "1.0.0", "Upload-Length", "4", "Upload-Metadata", tusMeta("filename", "more.txt")), 507)
}

func TestFileDropAcceptsFiveGiBAndRejectsLargerFiles(t *testing.T) {
	c := newClient(t)
	c.login()
	c.expect(c.do("POST", "/api/v1/files/folders", map[string]string{"path": "inbox"}), 201)
	link := c.folderLink("inbox", "upload")
	base := publicAPI(link)
	anonymous := &client{t: t, base: c.base, http: &http.Client{}, contract: c.contract}
	options := anonymous.expect(anonymous.do("OPTIONS", base+"/uploads", nil), 204)
	if got := options.header.Get("Tus-Max-Size"); got != "5368709120" {
		t.Fatalf("Tus-Max-Size = %s", got)
	}
	info := anonymous.expect(anonymous.do("GET", base, nil), 200)
	if !strings.Contains(string(info.body), `"maxUploadBytes":5368709120`) {
		t.Fatalf("link info = %s", info.body)
	}
	create := func(size string) response {
		return anonymous.do("POST", base+"/uploads", nil, "Tus-Resumable", "1.0.0", "Upload-Length", size, "Upload-Metadata", tusMeta("filename", "garden-archive.zip"))
	}
	anonymous.expect(create("5368709121"), 400)
	u := anonymous.expect(create("5368709120"), 201)
	location := u.header.Get("Location")
	head := anonymous.expect(anonymous.do("HEAD", location, nil, "Tus-Resumable", "1.0.0"), 200)
	if head.header.Get("Upload-Length") != "5368709120" {
		t.Fatalf("Upload-Length = %s", head.header.Get("Upload-Length"))
	}
	anonymous.expect(anonymous.do("PATCH", location, []byte("seed"), "Tus-Resumable", "1.0.0", "Upload-Offset", "0", "Content-Type", "application/offset+octet-stream"), 204)
	head = anonymous.expect(anonymous.do("HEAD", location, nil, "Tus-Resumable", "1.0.0"), 200)
	if head.header.Get("Upload-Offset") != "4" {
		t.Fatalf("Upload-Offset = %s", head.header.Get("Upload-Offset"))
	}
	anonymous.expect(anonymous.do("DELETE", location, nil, "Tus-Resumable", "1.0.0"), 204)
}
