package httpapi

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"

	"github.com/din9xtr/cloud-control-panel/internal/files"
	"github.com/din9xtr/cloud-control-panel/internal/share"
	"github.com/din9xtr/cloud-control-panel/internal/upload"
)

const (
	tusVersion     = "1.0.0"
	tusExtensions  = "creation,termination,expiration"
	tusContentType = "application/offset+octet-stream"
	uploadsPath    = "/api/v1/uploads/"
	maxMetadata    = 8 << 10
)

func (s *server) tusOptions(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	h.Set("Tus-Resumable", tusVersion)
	h.Set("Tus-Version", tusVersion)
	h.Set("Tus-Extension", tusExtensions)
	maxSize := upload.MaxSize
	if _, ok := r.Context().Value(uploadLinkKey{}).(share.Link); ok {
		maxSize = share.MaxUploadSize
	}
	h.Set("Tus-Max-Size", strconv.FormatInt(maxSize, 10))
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) tusCreate(w http.ResponseWriter, r *http.Request) {
	if !s.tusPreflight(w, r) {
		return
	}
	size, err := strconv.ParseInt(r.Header.Get("Upload-Length"), 10, 64)
	if err != nil {
		s.fail(w, r, badRequest("Upload-Length header is required; deferred length is not supported"))
		return
	}
	if size > upload.MaxSize {
		s.fail(w, r, upload.ErrTooLarge)
		return
	}
	meta, err := parseTusMetadata(r.Header.Get("Upload-Metadata"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	dir, err := files.ParsePath(meta["folder"])
	if err != nil {
		s.fail(w, r, err)
		return
	}
	var shareID string
	if link, ok := r.Context().Value(uploadLinkKey{}).(share.Link); ok {
		if !dir.IsRoot() || size > share.MaxUploadSize {
			s.fail(w, r, badRequest("upload links accept files up to 5 GiB in the destination folder only"))
			return
		}
		dir, shareID = link.Path, link.ID
	}
	u, err := s.uploads.Create(r.Context(), principalFrom(r.Context()).UserID, upload.CreateInput{
		ShareID: shareID,
		Size:    size,
		Dir:     dir,
		Name:    meta["filename"],
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	h := w.Header()
	location := uploadsPath + u.ID
	if shareID != "" {
		location = "/api/v1/public/links/" + r.PathValue("token") + "/uploads/" + u.ID
	}
	h.Set("Location", location)
	h.Set("Upload-Expires", u.ExpiresAt.Format(http.TimeFormat))
	h.Set("Upload-Offset", strconv.FormatInt(u.Offset, 10))
	w.WriteHeader(http.StatusCreated)
}

func (s *server) tusStatus(w http.ResponseWriter, r *http.Request) {
	if !s.tusPreflight(w, r) {
		return
	}
	u, err := s.uploads.Get(r.Context(), principalFrom(r.Context()).UserID, r.PathValue("id"))
	if err != nil {
		if api := classify(err); api != nil {
			w.WriteHeader(api.status)
			return
		}
		s.fail(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Cache-Control", "no-store")
	h.Set("Upload-Offset", strconv.FormatInt(u.Offset, 10))
	h.Set("Upload-Length", strconv.FormatInt(u.Size, 10))
	h.Set("Upload-Expires", u.ExpiresAt.Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
}

func (s *server) tusWrite(w http.ResponseWriter, r *http.Request) {
	if !s.tusPreflight(w, r) {
		return
	}
	if r.Header.Get("Content-Type") != tusContentType {
		s.fail(w, r, &apiError{status: http.StatusUnsupportedMediaType, code: "unsupported_media_type", message: "Content-Type must be " + tusContentType})
		return
	}
	offset, err := strconv.ParseInt(r.Header.Get("Upload-Offset"), 10, 64)
	if err != nil || offset < 0 {
		s.fail(w, r, badRequest("Upload-Offset header is required"))
		return
	}
	body := &idleReader{body: r.Body, control: http.NewResponseController(w), idle: s.guard.limits.UploadIdle}
	u, err := s.uploads.Write(r.Context(), principalFrom(r.Context()).UserID, r.PathValue("id"), offset, body)
	if err != nil {
		if classify(err) == nil && r.Context().Err() != nil {
			return
		}
		s.fail(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Upload-Offset", strconv.FormatInt(u.Offset, 10))
	if !u.Done() {
		h.Set("Upload-Expires", u.ExpiresAt.Format(http.TimeFormat))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) tusTerminate(w http.ResponseWriter, r *http.Request) {
	if !s.tusPreflight(w, r) {
		return
	}
	if err := s.uploads.Terminate(r.Context(), principalFrom(r.Context()).UserID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) tusPreflight(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Tus-Resumable", tusVersion)
	if r.Header.Get("Tus-Resumable") == tusVersion {
		return true
	}
	w.Header().Set("Tus-Version", tusVersion)
	s.fail(w, r, &apiError{status: http.StatusPreconditionFailed, code: "precondition_failed", message: "unsupported tus version"})
	return false
}

func parseTusMetadata(raw string) (map[string]string, error) {
	if len(raw) > maxMetadata {
		return nil, badRequest("Upload-Metadata is too large")
	}
	meta := make(map[string]string)
	for pair := range strings.SplitSeq(raw, ",") {
		key, encoded, _ := strings.Cut(strings.TrimSpace(pair), " ")
		if key == "" {
			continue
		}
		value, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, badRequest("Upload-Metadata values must be base64")
		}
		meta[key] = string(value)
	}
	if meta["filename"] == "" {
		return nil, badRequest("filename metadata is required")
	}
	return meta, nil
}
