package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/din9xtr/cloud-control-panel/internal/auth"
	"github.com/din9xtr/cloud-control-panel/internal/files"
	"github.com/din9xtr/cloud-control-panel/internal/share"
)

type uploadLinkKey struct{}

type publicLinkDTO struct {
	Name           string     `json:"name"`
	Mode           share.Mode `json:"mode"`
	ExpiresAt      time.Time  `json:"expiresAt"`
	MaxUploadBytes int64      `json:"maxUploadBytes"`
	Folder         *folderDTO `json:"folder,omitempty"`
}

func publicHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
}

func (s *server) publicFolder(w http.ResponseWriter, r *http.Request) {
	publicHeaders(w)
	link, err := s.shares.Resolve(r.Context(), r.PathValue("token"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if link.Kind != files.KindDir {
		s.fail(w, r, share.ErrLinkNotFound)
		return
	}
	out := publicLinkDTO{Name: link.Path.Name(), Mode: link.Mode, ExpiresAt: link.ExpiresAt, MaxUploadBytes: share.MaxUploadSize}
	if link.Mode == share.ModeDownload {
		_, dir, err := s.shares.ResolvePath(r.Context(), link.Token, r.URL.Query().Get("path"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		entries, err := s.files.ListShared(r.Context(), link.UserID, link.Path, dir)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		out.Folder = &folderDTO{Path: dir.String(), Entries: toEntries(entries, nil)}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *server) publicFolderContent(w http.ResponseWriter, r *http.Request) {
	publicHeaders(w)
	link, p, err := s.shares.ResolvePath(r.Context(), r.PathValue("token"), r.URL.Query().Get("path"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	f, entry, err := s.files.OpenShared(r.Context(), link.UserID, link.Path, p)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	defer f.Close()
	if r.Method == http.MethodGet && isFirstRange(r.Header.Get("Range")) {
		s.recordPublicDownload(r, link)
	}
	serveFile(w, entry, false)
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, entry.Name(), entry.ModTime, f)
}

func (s *server) publicFolderArchive(w http.ResponseWriter, r *http.Request) {
	publicHeaders(w)
	relatives := r.URL.Query()["path"]
	if len(relatives) == 0 {
		relatives = []string{""}
	}
	if len(relatives) > maxBatchPaths {
		s.fail(w, r, badRequest("too many paths"))
		return
	}
	var link share.Link
	paths := make([]files.Path, 0, len(relatives))
	for _, relative := range relatives {
		resolved, p, err := s.shares.ResolvePath(r.Context(), r.PathValue("token"), relative)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if _, err := s.files.StatShared(r.Context(), resolved.UserID, resolved.Path, p); err != nil {
			s.fail(w, r, err)
			return
		}
		link = resolved
		paths = append(paths, p)
	}
	release, err := acquire(r.Context(), s.guard.archives, 2*time.Second)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	defer release()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", disposition("attachment", link.Path.Name()+".zip"))
	if r.Method == http.MethodHead {
		return
	}
	s.recordPublicDownload(r, link)
	if err := s.files.WriteSharedArchive(r.Context(), link.UserID, link.Path, paths, w); err != nil {
		if r.Context().Err() == nil {
			s.log.WarnContext(r.Context(), "archive aborted", slog.String("error", redact(err)))
		}
		panic(http.ErrAbortHandler)
	}
}

func (s *server) recordPublicDownload(r *http.Request, link share.Link) {
	if err := s.shares.RecordDownload(r.Context(), link.ID); err != nil {
		s.log.WarnContext(r.Context(), "record download", slog.String("error", redact(err)))
	}
}

func (s *server) requireUploadLink(next http.HandlerFunc) http.HandlerFunc {
	return s.throttle(s.guard.public, func(w http.ResponseWriter, r *http.Request) {
		publicHeaders(w)
		link, err := s.shares.Resolve(r.Context(), r.PathValue("token"))
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if link.Mode != share.ModeUpload || link.Kind != files.KindDir {
			s.fail(w, r, share.ErrLinkNotFound)
			return
		}
		if id := r.PathValue("id"); id != "" {
			u, err := s.uploads.Get(r.Context(), link.UserID, id)
			if err != nil {
				s.fail(w, r, err)
				return
			}
			if u.ShareID != link.ID || u.Dir != link.Path {
				s.fail(w, r, share.ErrLinkNotFound)
				return
			}
		}
		ctx := context.WithValue(r.Context(), principalKey{}, auth.Principal{UserID: link.UserID})
		ctx = context.WithValue(ctx, uploadLinkKey{}, link)
		next(w, r.WithContext(ctx))
	})
}
