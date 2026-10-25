package web

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:dist
var dist embed.FS

var _contentTypes = map[string]string{
	".js":          "text/javascript; charset=utf-8",
	".css":         "text/css; charset=utf-8",
	".webmanifest": "application/manifest+json",
	".ico":         "image/x-icon",
	".svg":         "image/svg+xml",
	".woff2":       "font/woff2",
}

type Handler struct {
	assets fs.FS
	index  []byte
	files  http.Handler
}

func NewHandler() (*Handler, error) {
	assets, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, err
	}
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		index = nil
	}
	return &Handler{assets: assets, index: index, files: http.FileServerFS(assets)}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name != "" && name != "index.html" && h.exists(name) {
		if contentType, ok := _contentTypes[path.Ext(name)]; ok {
			w.Header().Set("Content-Type", contentType)
		}
		if strings.HasPrefix(name, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		h.files.ServeHTTP(w, r)
		return
	}
	if h.index == nil || path.Ext(name) != "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(h.index))
}

func (h *Handler) exists(name string) bool {
	info, err := fs.Stat(h.assets, name)
	return err == nil && !info.IsDir()
}
