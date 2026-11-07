package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/auth"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store"
)

// SessionCookieName is deliberately not "session": a generic name collides
// with anything else served from the same host.
const SessionCookieName = "oltmon_session"

type ctxKey string

const userCtxKey ctxKey = "user"

// WithAuth enables the login routes. Without it those routes answer 501, which
// is preferable to a login endpoint that half exists.
func (s *Server) WithAuth(authStore store.AuthStore) *Server {
	s.auth = authStore
	return s
}

// WithAudit enables audit recording. Without it, actions still work but leave
// no trail — acceptable in tests, not in production.
func (s *Server) WithAudit(audit store.AuditStore) *Server {
	s.audit = audit
	return s
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type sessionUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	FullName string `json:"fullName"`
	Role     string `json:"role"`
}

func toSessionUser(user store.User) sessionUser {
	return sessionUser{ID: user.ID, Username: user.Username, FullName: user.FullName, Role: user.Role}
}

func (s *Server) requireAuthConfigured(w http.ResponseWriter) bool {
	if s.auth == nil {
		writeError(w, http.StatusNotImplemented, "auth_disabled", "authentication is not configured")
		return false
	}
	return true
}

// login answers with the same 401 and the same wording for an unknown user, a
// wrong password and a disabled account. Anything more specific is a user
// enumeration oracle.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuthConfigured(w) {
		return
	}

	var body loginRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "expected {username, password}")
		return
	}

	user, err := s.auth.UserByUsername(r.Context(), body.Username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// Spend the same time as a real check before failing.
			auth.BurnTime(body.Password)
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
			return
		}
		s.handleStoreError(w, err)
		return
	}

	ok, err := auth.VerifyPassword(body.Password, user.PasswordHash)
	if err != nil || !ok || user.Disabled {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}

	token, hash, err := auth.NewSessionToken()
	if err != nil {
		s.log.Error("session token", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not start a session")
		return
	}

	expiresAt := time.Now().Add(auth.SessionTTL)
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	if err := s.auth.CreateSession(r.Context(), user.ID, hash, expiresAt, ip, r.UserAgent()); err != nil {
		s.handleStoreError(w, err)
		return
	}
	if err := s.auth.TouchLogin(r.Context(), user.ID); err != nil {
		s.log.Warn("touch login", "error", err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		// Secure whenever the request arrived over TLS, or through a proxy that
		// says it did. A plain-HTTP deployment stays broken rather than sending
		// the cookie in the clear.
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt,
		MaxAge:   int(auth.SessionTTL / time.Second),
	})

	writeJSON(w, http.StatusOK, toSessionUser(user))
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if !s.requireAuthConfigured(w) {
		return
	}

	if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
		if err := s.auth.RevokeSession(r.Context(), auth.HashToken(cookie.Value)); err != nil {
			s.log.Warn("revoke session", "error", err)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := userFrom(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in first")
		return
	}
	writeJSON(w, http.StatusOK, toSessionUser(user))
}

// requireSession protects a route. It is the only place a cookie becomes a
// user, so every protected handler sees the same rules.
func (s *Server) requireSession(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil {
			writeError(w, http.StatusNotImplemented, "auth_disabled", "authentication is not configured")
			return
		}

		cookie, err := r.Cookie(SessionCookieName)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in first")
			return
		}

		user, err := s.auth.UserBySession(r.Context(), auth.HashToken(cookie.Value))
		if err != nil {
			// Expired, revoked and never-existed are one answer to a client.
			writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in first")
			return
		}

		next(w, r.WithContext(context.WithValue(r.Context(), userCtxKey, user)))
	}
}

// passwordResetRequest records the intent and answers identically whether or
// not the address exists. There is no mailer yet, so the request lands in the
// audit log where an administrator can act on it with `userctl passwd`.
//
// It never says "no such account": that would turn this endpoint into an
// account-enumeration oracle.
func (s *Server) passwordResetRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "expected {email}")
		return
	}

	if body.Email != "" && s.audit != nil {
		if err := s.audit.RecordAudit(r.Context(), "anonymous", "password_reset_request", body.Email, ""); err != nil {
			s.log.Warn("audit reset request", "error", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func userFrom(ctx context.Context) (store.User, bool) {
	user, ok := ctx.Value(userCtxKey).(store.User)
	return user, ok
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return r.Header.Get("X-Forwarded-Proto") == "https"
}
