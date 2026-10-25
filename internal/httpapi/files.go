package httpapi

import (
	"context"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/din9xtr/cloud-control-panel/internal/files"
	"github.com/din9xtr/cloud-control-panel/internal/share"
)

const (
	searchLimit     = 200
	maxBatchPaths   = 1000
	sandboxCSP      = "sandbox; default-src 'none'; img-src 'self'; media-src 'self'; style-src 'unsafe-inline'"
	maxSearchLength = 255
)

func (s *server) listFolder(w http.ResponseWriter, r *http.Request) {
	userID := principalFrom(r.Context()).UserID
	dir, err := files.ParsePath(r.URL.Query().Get("path"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	entries, err := s.files.List(r.Context(), userID, dir)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	links, err := s.linksByPath(r, userID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, folderDTO{Path: dir.String(), Entries: toEntries(entries, links)})
}

func (s *server) searchFiles(w http.ResponseWriter, r *http.Request) {
	userID := principalFrom(r.Context()).UserID
	query := r.URL.Query()
	q := strings.TrimSpace(query.Get("q"))
	if q == "" || len(q) > maxSearchLength {
		s.fail(w, r, badRequest("query must be 1-255 characters"))
		return
	}
	dir, err := files.ParsePath(query.Get("path"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	release, err := acquire(r.Context(), s.guard.searches, 2*time.Second)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	defer release()
	ctx, cancel := context.WithTimeout(r.Context(), s.guard.limits.SearchTimeout)
	defer cancel()
	found, err := s.files.Search(ctx, userID, dir, q, searchLimit)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	links, err := s.linksByPath(r, userID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, entryListDTO{Entries: toEntries(found, links)})
}

type pathRequest struct {
	Path string `json:"path"`
}

func (s *server) createFolder(w http.ResponseWriter, r *http.Request) {
	var req pathRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.fail(w, r, err)
		return
	}
	p, err := files.ParsePath(req.Path)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	entry, err := s.files.CreateFolder(r.Context(), principalFrom(r.Context()).UserID, p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toEntry(entry, nil))
}

type renameRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

func (s *server) renameEntry(w http.ResponseWriter, r *http.Request) {
	var req renameRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.fail(w, r, err)
		return
	}
	p, err := files.ParsePath(req.Path)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	entry, err := s.files.Rename(r.Context(), principalFrom(r.Context()).UserID, p, req.Name)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toEntry(entry, nil))
}

type moveRequest struct {
	Paths       []string `json:"paths"`
	Destination string   `json:"destination"`
}

func (s *server) moveEntries(w http.ResponseWriter, r *http.Request) {
	var req moveRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.fail(w, r, err)
		return
	}
	paths, err := parsePaths(req.Paths)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	dest, err := files.ParsePath(req.Destination)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.files.Move(r.Context(), principalFrom(r.Context()).UserID, paths, dest); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type pathsRequest struct {
	Paths []string `json:"paths"`
}

func (s *server) trashEntries(w http.ResponseWriter, r *http.Request) {
	var req pathsRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.fail(w, r, err)
		return
	}
	paths, err := parsePaths(req.Paths)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.files.Trash(r.Context(), principalFrom(r.Context()).UserID, paths); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) downloadFile(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query().Get("path")
	if raw == "" {
		s.fail(w, r, badRequest("path is required"))
		return
	}
	p, err := files.ParsePath(raw)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	f, entry, err := s.files.Open(r.Context(), principalFrom(r.Context()).UserID, p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	defer f.Close()

	inline, _ := strconv.ParseBool(r.URL.Query().Get("inline"))
	serveFile(w, entry, inline)
	http.ServeContent(w, r, entry.Name(), entry.ModTime, f)
}

func (s *server) downloadArchive(w http.ResponseWriter, r *http.Request) {
	raw := r.URL.Query()["path"]
	if len(raw) == 0 {
		s.fail(w, r, badRequest("at least one path is required"))
		return
	}
	paths, err := parsePaths(raw)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	userID := principalFrom(r.Context()).UserID
	for _, p := range paths {
		if _, err := s.files.Stat(r.Context(), userID, p); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	release, err := acquire(r.Context(), s.guard.archives, 2*time.Second)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	defer release()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", disposition("attachment", files.ArchiveName(paths)))
	w.Header().Set("Cache-Control", "no-store")
	if err := s.files.WriteArchive(r.Context(), userID, paths, w); err != nil {
		if r.Context().Err() == nil {
			s.log.WarnContext(r.Context(), "archive aborted", slog.String("error", redact(err)))
		}
		panic(http.ErrAbortHandler)
	}
}

func serveFile(w http.ResponseWriter, entry files.Entry, inline bool) {
	contentType := entry.ContentType()
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", "private, no-cache")
	if inline && inlineAllowed(contentType) {
		h.Set("Content-Disposition", disposition("inline", entry.Name()))
		h.Set("X-Frame-Options", "SAMEORIGIN")
		if !strings.HasPrefix(contentType, "application/pdf") {
			h.Set("Content-Security-Policy", sandboxCSP)
		} else {
			h.Set("Content-Security-Policy", "frame-ancestors 'self'")
		}
		return
	}
	h.Set("Content-Disposition", disposition("attachment", entry.Name()))
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
}

func inlineAllowed(contentType string) bool {
	switch {
	case strings.HasPrefix(contentType, "image/svg"):
		return false
	case strings.HasPrefix(contentType, "image/"),
		strings.HasPrefix(contentType, "video/"),
		strings.HasPrefix(contentType, "audio/"),
		strings.HasPrefix(contentType, "application/pdf"),
		strings.HasPrefix(contentType, "text/plain"):
		return true
	}
	return false
}

func disposition(kind, name string) string {
	if v := mime.FormatMediaType(kind, map[string]string{"filename": name}); v != "" {
		return v
	}
	return kind
}

func parsePaths(raw []string) ([]files.Path, error) {
	if len(raw) == 0 || len(raw) > maxBatchPaths {
		return nil, badRequest("between 1 and 1000 paths are required")
	}
	out := make([]files.Path, 0, len(raw))
	for _, r := range raw {
		p, err := files.ParsePath(r)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *server) linksByPath(r *http.Request, userID int64) (map[string]share.Link, error) {
	links, err := s.shares.List(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	byPath := make(map[string]share.Link, len(links))
	for _, l := range links {
		if current, ok := byPath[l.Path.String()]; !ok || l.ExpiresAt.After(current.ExpiresAt) {
			byPath[l.Path.String()] = l
		}
	}
	return byPath, nil
}
