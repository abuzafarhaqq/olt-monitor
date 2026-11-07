package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/auth"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store/postgres"
)

// WithUsers enables account administration. Without it the routes answer 501,
// so a partially configured server cannot half-manage accounts.
func (s *Server) WithUsers(users store.UserStore) *Server {
	s.users = users
	return s
}

// userView is what leaves the server: never the hash.
type userView struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	FullName    string     `json:"fullName"`
	Email       string     `json:"email"`
	Phone       *string    `json:"phone"`
	Role        string     `json:"role"`
	Disabled    bool       `json:"disabled"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
}

func toUserView(user store.User) userView {
	return userView{
		ID: user.ID, Username: user.Username, FullName: user.FullName, Email: user.Email,
		Phone: user.Phone, Role: user.Role, Disabled: user.Disabled, LastLoginAt: user.LastLoginAt,
	}
}

func (s *Server) requireUsers(w http.ResponseWriter) bool {
	if s.users == nil {
		writeError(w, http.StatusNotImplemented, "users_disabled", "user management is not configured")
		return false
	}
	return true
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	if !s.requireUsers(w) {
		return
	}
	users, err := s.users.ListUsers(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	views := make([]userView, 0, len(users))
	for _, user := range users {
		views = append(views, toUserView(user))
	}
	writeJSON(w, http.StatusOK, views)
}

type createUserBody struct {
	Username string `json:"username"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireUsers(w) {
		return
	}

	var body createUserBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "could not read the account")
		return
	}

	role, ok := postgres.NormaliseRole(body.Role)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_role", "role must be administrator, network-engineer, noc-operator or viewer")
		return
	}
	if strings.TrimSpace(body.Username) == "" || strings.TrimSpace(body.FullName) == "" || strings.TrimSpace(body.Email) == "" {
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
		Phone:        strings.TrimSpace(body.Phone),
		Role:         role,
		PasswordHash: hash,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			writeError(w, http.StatusConflict, "duplicate", "that username or email already exists")
			return
		}
		s.handleStoreError(w, err)
		return
	}

	s.auditActor(r, "user.create", created.ID, created.Username)
	writeJSON(w, http.StatusCreated, toUserView(created))
}

type updateUserBody struct {
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Role     string `json:"role"`
	Disabled bool   `json:"disabled"`
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireUsers(w) {
		return
	}

	id := r.PathValue("id")
	var body updateUserBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "could not read the account")
		return
	}

	role, ok := postgres.NormaliseRole(body.Role)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_role", "unknown role")
		return
	}

	current, err := s.users.ListUsers(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	var target *store.User
	for i := range current {
		if current[i].ID == id {
			target = &current[i]
		}
	}
	if target == nil {
		writeError(w, http.StatusNotFound, "not_found", "no such account")
		return
	}

	// Losing the last administrator is unrecoverable without shell access.
	demoting := target.Role == "administrator" && (role != "administrator" || body.Disabled)
	if demoting {
		admins, err := s.users.CountEnabledAdmins(r.Context())
		if err != nil {
			s.handleStoreError(w, err)
			return
		}
		if admins <= 1 {
			writeError(w, http.StatusConflict, "last_admin", store.ErrLastAdmin.Error())
			return
		}
	}

	updated, err := s.users.UpdateUser(r.Context(), store.UserWrite{
		ID: id, FullName: strings.TrimSpace(body.FullName), Email: strings.TrimSpace(body.Email),
		Phone: strings.TrimSpace(body.Phone), Role: role, Disabled: body.Disabled,
	})
	if err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "duplicate", "that email already exists")
			return
		}
		s.handleStoreError(w, err)
		return
	}

	s.auditActor(r, "user.update", id, updated.Username)
	writeJSON(w, http.StatusOK, toUserView(updated))
}

func (s *Server) setUserPassword(w http.ResponseWriter, r *http.Request) {
	if !s.requireUsers(w) {
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "expected {password}")
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, "weak_password", err.Error())
		return
	}

	id := r.PathValue("id")
	if err := s.users.SetPasswordHash(r.Context(), id, hash); err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.auditActor(r, "user.password.reset", id, "")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireUsers(w) {
		return
	}

	id := r.PathValue("id")
	actor, _ := userFrom(r.Context())
	if actor.ID == id {
		writeError(w, http.StatusConflict, "self_delete", "an account cannot delete itself")
		return
	}

	users, err := s.users.ListUsers(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	var target *store.User
	for i := range users {
		if users[i].ID == id {
			target = &users[i]
		}
	}
	if target == nil {
		writeError(w, http.StatusNotFound, "not_found", "no such account")
		return
	}
	if target.Role == "administrator" && !target.Disabled {
		admins, err := s.users.CountEnabledAdmins(r.Context())
		if err != nil {
			s.handleStoreError(w, err)
			return
		}
		if admins <= 1 {
			writeError(w, http.StatusConflict, "last_admin", store.ErrLastAdmin.Error())
			return
		}
	}

	if err := s.users.DeleteUser(r.Context(), id); err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.auditActor(r, "user.delete", id, target.Username)
	w.WriteHeader(http.StatusNoContent)
}
