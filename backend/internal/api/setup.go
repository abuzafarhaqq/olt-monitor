package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/auth"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store/postgres"
)

// setupStatus tells the UI whether the platform has ever been initialised.
//
// Unauthenticated on purpose: before the first account exists there is nobody to
// authenticate as. It reveals one bit — "is there a user" — and nothing else.
func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) {
	if s.users == nil {
		writeError(w, http.StatusServiceUnavailable, "users_disabled", "user management is not configured")
		return
	}
	users, err := s.users.ListUsers(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"required": len(users) == 0})
}

type setupBody struct {
	Username string `json:"username"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// setup creates the first administrator and signs them in.
//
// The only way in is an empty user table: once one account exists this endpoint
// refuses forever, so it cannot become a second front door.
func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	if s.users == nil {
		writeError(w, http.StatusServiceUnavailable, "users_disabled", "user management is not configured")
		return
	}

	users, err := s.users.ListUsers(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	if len(users) > 0 {
		writeError(w, http.StatusConflict, "already_initialised", "this platform already has accounts")
		return
	}

	var body setupBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "could not read the account")
		return
	}
	if strings.TrimSpace(body.Username) == "" || strings.TrimSpace(body.FullName) == "" ||
		strings.TrimSpace(body.Email) == "" {
		writeError(w, http.StatusBadRequest, "invalid_user", "username, full name and email are required")
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, "weak_password", err.Error())
		return
	}

	created, err := s.users.CreateUser(r.Context(), store.CreateUserParams{
		Username:     strings.TrimSpace(body.Username),
		FullName:     strings.TrimSpace(body.FullName),
		Email:        strings.TrimSpace(body.Email),
		Role:         postgres.NormaliseRoleOrAdmin(),
		PasswordHash: hash,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "create_failed", err.Error())
		return
	}

	s.auditActor(r, "user.bootstrap", created.ID, created.Username)
	if err := s.issueSession(w, r, created); err != nil {
		// The account exists; signing in can be done from the login page.
		s.log.Warn("issue session after setup", "error", err)
	}
	writeJSON(w, http.StatusCreated, toSessionUser(created))
}
