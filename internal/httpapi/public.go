package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/din9xtr/cloud-control-panel/internal/share"
)

var _tokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{22}$`)

const missingLinkStyle = `
body{margin:0;min-height:100vh;display:grid;place-items:center;background:#0D1117;color:#E6EDF3;
font:15px/1.5 -apple-system,BlinkMacSystemFont,Inter,system-ui,sans-serif}
main{padding:24px;text-align:center;display:grid;gap:8px}
samp{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;color:#7D8590;font-size:13px}
h1{margin:0;font-size:20px;font-weight:600}p{margin:0;color:#A6ADB8}
`

var (
	_missingLinkPage = []byte(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex"><title>Link unavailable</title>
<style>` + missingLinkStyle + `</style></head>
<body><main><samp>404 · link</samp><h1>This link is no longer available</h1>
<p>It may have expired or been revoked by its owner.</p></main></body></html>`)
	_missingLinkCSP = "default-src 'none'; style-src " + styleHash(missingLinkStyle)
)

func styleHash(style string) string {
	sum := sha256.Sum256([]byte(style))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

func (s *server) publicDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	w.Header().Set("Referrer-Policy", "no-referrer")
	token := r.PathValue("token")
	if !_tokenPattern.MatchString(token) {
		s.missingLink(w)
		return
	}
	f, entry, link, err := s.shares.Open(r.Context(), token)
	if errors.Is(err, share.ErrLinkNotFound) {
		s.missingLink(w)
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	defer f.Close()

	if r.Method == http.MethodGet && isFirstRange(r.Header.Get("Range")) {
		if err := s.shares.RecordDownload(r.Context(), link.ID); err != nil {
			s.log.WarnContext(r.Context(), "record download", slog.String("error", redact(err)))
		}
	}
	serveFile(w, entry, false)
	http.ServeContent(w, r, entry.Name(), entry.ModTime, f)
}

func (s *server) missingLink(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", _missingLinkCSP)
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(_missingLinkPage)
}

func isFirstRange(header string) bool {
	return header == "" || strings.HasPrefix(header, "bytes=0-")
}
