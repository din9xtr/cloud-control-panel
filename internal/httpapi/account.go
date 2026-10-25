package httpapi

import (
	"errors"
	"net/http"

	"github.com/din9xtr/cloud-control-panel/internal/auth"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.fail(w, r, err)
		return
	}
	if req.Username == "" || req.Password == "" || len(req.Username) > 64 || len(req.Password) > 128 {
		s.fail(w, r, badRequest("username and password are required"))
		return
	}
	res, err := s.auth.Login(r.Context(), auth.LoginInput{
		Username:  req.Username,
		Password:  req.Password,
		IP:        s.clientIP(r),
		Network:   s.clientNetwork(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		s.logLoginFailure(r, err)
		s.fail(w, r, err)
		return
	}
	user, err := s.auth.User(r.Context(), res.Principal.UserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.setSessionCookie(w, r, res.Token, res.ExpiresAt)
	writeJSON(w, http.StatusOK, toMe(user))
}

func (s *server) logLoginFailure(r *http.Request, err error) {
	var throttled *auth.ThrottledError
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		s.log.WarnContext(r.Context(), "login failed", "ip", s.clientIP(r), "reason", "invalid_credentials")
	case errors.As(err, &throttled):
		s.log.WarnContext(r.Context(), "login failed", "ip", s.clientIP(r), "reason", "throttled")
	}
}

type setupStatusDTO struct {
	Required bool `json:"required"`
}

func (s *server) setupStatus(w http.ResponseWriter, r *http.Request) {
	required, err := s.auth.SetupRequired(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, setupStatusDTO{Required: required})
}

func (s *server) setup(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.fail(w, r, err)
		return
	}
	res, err := s.auth.Setup(r.Context(), auth.SetupInput{
		Username:  req.Username,
		Password:  req.Password,
		IP:        s.clientIP(r),
		UserAgent: r.UserAgent(),
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.files.PrepareUser(r.Context(), res.Principal.UserID); err != nil {
		s.fail(w, r, err)
		return
	}
	user, err := s.auth.User(r.Context(), res.Principal.UserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.setSessionCookie(w, r, res.Token, res.ExpiresAt)
	writeJSON(w, http.StatusCreated, toMe(user))
}

func (s *server) logout(w http.ResponseWriter, r *http.Request) {
	if err := s.auth.Logout(r.Context(), principalFrom(r.Context())); err != nil && !errors.Is(err, auth.ErrSessionNotFound) {
		s.fail(w, r, err)
		return
	}
	s.clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	user, err := s.auth.User(r.Context(), principalFrom(r.Context()).UserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toMe(user))
}

type changeCredentialsRequest struct {
	CurrentPassword string `json:"currentPassword"`
	Username        string `json:"username"`
	NewPassword     string `json:"newPassword"`
}

func (s *server) changeCredentials(w http.ResponseWriter, r *http.Request) {
	var req changeCredentialsRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.fail(w, r, err)
		return
	}
	user, err := s.auth.ChangeCredentials(r.Context(), principalFrom(r.Context()), auth.ChangeCredentialsInput(req))
	if errors.Is(err, auth.ErrInvalidCredentials) {
		s.fail(w, r, &apiError{status: http.StatusForbidden, code: "invalid_credentials", message: "current password is incorrect"})
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toMe(user))
}

func (s *server) listSessions(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	sessions, err := s.auth.Sessions(r.Context(), p.UserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	out := make([]sessionDTO, 0, len(sessions))
	for _, sess := range sessions {
		out = append(out, sessionDTO{
			ID:         sess.ID,
			Current:    sess.ID == p.SessionID,
			CreatedAt:  sess.CreatedAt,
			LastSeenAt: sess.LastSeenAt,
			IP:         sess.IP,
			UserAgent:  sess.UserAgent,
		})
	}
	writeJSON(w, http.StatusOK, newItems(out))
}

func (s *server) revokeSession(w http.ResponseWriter, r *http.Request) {
	p := principalFrom(r.Context())
	if err := s.auth.RevokeSession(r.Context(), p.UserID, r.PathValue("id")); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
