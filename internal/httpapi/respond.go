package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"

	"github.com/din9xtr/cloud-control-panel/internal/auth"
	"github.com/din9xtr/cloud-control-panel/internal/files"
	"github.com/din9xtr/cloud-control-panel/internal/settings"
	"github.com/din9xtr/cloud-control-panel/internal/share"
	"github.com/din9xtr/cloud-control-panel/internal/upload"
)

const maxJSONBody = 1 << 20

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type apiError struct {
	status  int
	code    string
	message string
}

func (e *apiError) Error() string {
	return e.message
}

func badRequest(message string) error {
	return &apiError{status: http.StatusBadRequest, code: "invalid_request", message: message}
}

type mapping struct {
	target error
	status int
	code   string
}

var _errorMappings = []mapping{
	{errBusy, http.StatusServiceUnavailable, "busy"},
	{auth.ErrBusy, http.StatusServiceUnavailable, "busy"},
	{upload.ErrTooManyUploads, http.StatusTooManyRequests, "too_many_requests"},
	{files.ErrInvalidPath, http.StatusBadRequest, "invalid_request"},
	{files.ErrInvalidName, http.StatusBadRequest, "invalid_request"},
	{files.ErrRootOperation, http.StatusBadRequest, "invalid_request"},
	{files.ErrMoveIntoSelf, http.StatusBadRequest, "invalid_request"},
	{files.ErrNotDirectory, http.StatusBadRequest, "invalid_request"},
	{files.ErrIsDirectory, http.StatusBadRequest, "invalid_request"},
	{files.ErrNotFound, http.StatusNotFound, "not_found"},
	{files.ErrTrashItemNotFound, http.StatusNotFound, "not_found"},
	{files.ErrExists, http.StatusConflict, "conflict"},
	{files.ErrQuotaExceeded, http.StatusInsufficientStorage, "quota_exceeded"},
	{files.ErrDiskFull, http.StatusInsufficientStorage, "disk_full"},
	{auth.ErrUnauthenticated, http.StatusUnauthorized, "unauthenticated"},
	{auth.ErrInvalidCredentials, http.StatusUnauthorized, "invalid_credentials"},
	{auth.ErrInvalidUsername, http.StatusBadRequest, "invalid_request"},
	{auth.ErrWeakPassword, http.StatusBadRequest, "invalid_request"},
	{auth.ErrNothingToChange, http.StatusBadRequest, "invalid_request"},
	{auth.ErrUsernameTaken, http.StatusConflict, "conflict"},
	{auth.ErrAlreadySetUp, http.StatusConflict, "conflict"},
	{auth.ErrSessionNotFound, http.StatusNotFound, "not_found"},
	{auth.ErrUserNotFound, http.StatusUnauthorized, "unauthenticated"},
	{share.ErrLinkNotFound, http.StatusNotFound, "not_found"},
	{share.ErrInvalidTTL, http.StatusBadRequest, "invalid_request"},
	{share.ErrInvalidMode, http.StatusBadRequest, "invalid_request"},
	{share.ErrNotAFile, http.StatusBadRequest, "invalid_request"},
	{settings.ErrInvalidQuota, http.StatusBadRequest, "invalid_request"},
	{settings.ErrInvalidTrashRetention, http.StatusBadRequest, "invalid_request"},
	{settings.ErrInvalidLinkTTL, http.StatusBadRequest, "invalid_request"},
	{upload.ErrNotFound, http.StatusNotFound, "not_found"},
	{upload.ErrInvalidSize, http.StatusBadRequest, "invalid_request"},
	{upload.ErrOffsetMismatch, http.StatusConflict, "conflict"},
	{upload.ErrLocked, http.StatusLocked, "locked"},
	{upload.ErrTooLarge, http.StatusRequestEntityTooLarge, "too_large"},
}

func classify(err error) *apiError {
	var api *apiError
	if errors.As(err, &api) {
		return api
	}
	var throttled *auth.ThrottledError
	if errors.As(err, &throttled) {
		return &apiError{status: http.StatusTooManyRequests, code: "too_many_requests", message: throttled.Error()}
	}
	for _, m := range _errorMappings {
		if errors.Is(err, m.target) {
			return &apiError{status: m.status, code: m.code, message: m.target.Error()}
		}
	}
	return nil
}

func (s *server) fail(w http.ResponseWriter, r *http.Request, err error) {
	if api := classify(err); api != nil {
		var throttled *auth.ThrottledError
		if errors.As(err, &throttled) {
			w.Header().Set("Retry-After", strconv.Itoa(int(throttled.RetryAfter.Seconds())+1))
		}
		writeError(w, api.status, api.code, api.message)
		return
	}
	if errors.Is(err, context.Canceled) && r.Context().Err() != nil {
		return
	}
	s.log.ErrorContext(r.Context(), "request failed", slog.String("route", r.Pattern), slog.String("error", redact(err)))
	writeError(w, http.StatusInternalServerError, "internal", "internal server error")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mediaType != "application/json" {
		return &apiError{status: http.StatusUnsupportedMediaType, code: "unsupported_media_type", message: "Content-Type must be application/json"}
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return badRequest("malformed JSON body")
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return badRequest("body must contain a single JSON object")
	}
	return nil
}
