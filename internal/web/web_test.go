package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func newTestHandler() *Handler {
	assets := fstest.MapFS{
		"index.html":           {Data: []byte("<!doctype html><title>app</title>")},
		"manifest.webmanifest": {Data: []byte("{}")},
		"assets/app-1a2b.js":   {Data: []byte("console.log(1)")},
		"assets/app-1a2b.css":  {Data: []byte("body{margin:0}")},
	}
	return &Handler{assets: assets, index: assets["index.html"].Data, files: http.FileServerFS(assets)}
}

func TestServesStaticFilesWithTypesAndCaching(t *testing.T) {
	h := newTestHandler()
	tests := []struct {
		path, contentType, cache string
	}{
		{path: "/manifest.webmanifest", contentType: "application/manifest+json", cache: "no-cache"},
		{path: "/assets/app-1a2b.js", contentType: "text/javascript; charset=utf-8", cache: "public, max-age=31536000, immutable"},
		{path: "/assets/app-1a2b.css", contentType: "text/css; charset=utf-8", cache: "public, max-age=31536000, immutable"},
		{path: "/files/garden/greenhouse", contentType: "text/html; charset=utf-8", cache: "no-cache"},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tt.path, rec.Code)
		}
		if got := rec.Header().Get("Content-Type"); got != tt.contentType {
			t.Errorf("%s Content-Type = %q, want %q", tt.path, got, tt.contentType)
		}
		if got := rec.Header().Get("Cache-Control"); got != tt.cache {
			t.Errorf("%s Cache-Control = %q, want %q", tt.path, got, tt.cache)
		}
	}
}

func TestMissingAssetIsNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}
