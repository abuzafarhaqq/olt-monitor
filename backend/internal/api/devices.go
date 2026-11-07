package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/auth"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/secrets"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store"
)

// WithDevices enables the write endpoints for the inventory.
func (s *Server) WithDevices(devices store.DeviceStore) *Server {
	s.devices = devices
	return s
}

// WithSecrets enables credential storage. Without it those routes answer 503 —
// better than accepting a password we cannot encrypt.
func (s *Server) WithSecrets(service *secrets.Service) *Server {
	s.secrets = service
	return s
}

// guard is requireSession plus a permission check. The browser hides controls;
// this is the enforcement.
func (s *Server) guard(permission auth.Permission, next http.HandlerFunc) http.HandlerFunc {
	return s.requireSession(func(w http.ResponseWriter, r *http.Request) {
		user, ok := userFrom(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in first")
			return
		}
		if !auth.Can(user.Role, permission) {
			writeError(w, http.StatusForbidden, "forbidden", "your role cannot do this")
			return
		}
		next(w, r)
	})
}

type deviceBody struct {
	Name              string `json:"name"`
	Vendor            string `json:"vendor"`
	Model             string `json:"model"`
	Technology        string `json:"technology"`
	ManagementAddress string `json:"managementAddress"`
	ManagementPort    int    `json:"managementPort"`
	Connection        string `json:"connection"`
	Enabled           *bool  `json:"enabled"`
}

// validateDevice keeps every rule in one place, including the ones that look
// pedantic: a technology of "epon" in lower case silently misses the threshold
// table, and an address that is not an IP fails only when the collector dials.
func validateDevice(body deviceBody) (store.OltWrite, string) {
	write := store.OltWrite{
		Name:              strings.TrimSpace(body.Name),
		Vendor:            strings.TrimSpace(body.Vendor),
		Model:             strings.TrimSpace(body.Model),
		Technology:        strings.ToUpper(strings.TrimSpace(body.Technology)),
		ManagementAddress: strings.TrimSpace(body.ManagementAddress),
		ManagementPort:    body.ManagementPort,
		Connection:        strings.ToLower(strings.TrimSpace(body.Connection)),
		Enabled:           true,
	}
	if body.Enabled != nil {
		write.Enabled = *body.Enabled
	}

	if write.Name == "" || write.Vendor == "" || write.Model == "" {
		return write, "name, vendor and model are required"
	}
	if write.Technology != "EPON" && write.Technology != "GPON" {
		return write, "technology must be EPON or GPON"
	}
	if write.Connection != "ssh" && write.Connection != "telnet" {
		return write, "connection must be ssh or telnet"
	}
	if net.ParseIP(write.ManagementAddress) == nil {
		return write, "managementAddress must be an IP address"
	}
	if write.ManagementPort == 0 {
		if write.Connection == "ssh" {
			write.ManagementPort = 22
		} else {
			write.ManagementPort = 23
		}
	}
	if write.ManagementPort < 1 || write.ManagementPort > 65535 {
		return write, "managementPort must be between 1 and 65535"
	}
	return write, ""
}

func newOltID() (string, error) {
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "olt-" + hex.EncodeToString(raw), nil
}

func (s *Server) createOlt(w http.ResponseWriter, r *http.Request) {
	var body deviceBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "could not read the device")
		return
	}

	write, problem := validateDevice(body)
	if problem != "" {
		writeError(w, http.StatusBadRequest, "invalid_device", problem)
		return
	}

	id, err := newOltID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not allocate an id")
		return
	}
	write.ID = id

	created, err := s.devices.CreateOlt(r.Context(), write)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			writeError(w, http.StatusConflict, "duplicate", "an OLT with that name already exists")
			return
		}
		s.handleStoreError(w, err)
		return
	}

	s.auditActor(r, "olt.create", created.ID, created.Name)
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) updateOlt(w http.ResponseWriter, r *http.Request) {
	var body deviceBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "could not read the device")
		return
	}

	write, problem := validateDevice(body)
	if problem != "" {
		writeError(w, http.StatusBadRequest, "invalid_device", problem)
		return
	}
	write.ID = r.PathValue("id")

	updated, err := s.devices.UpdateOlt(r.Context(), write)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.auditActor(r, "olt.update", updated.ID, updated.Name)
	writeJSON(w, http.StatusOK, updated)
}

type credentialBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// setCredential is write-only. There is no response body carrying the secret, no
// GET that returns it, and no "show password" for any role.
func (s *Server) setCredential(w http.ResponseWriter, r *http.Request) {
	if s.secrets == nil {
		writeError(w, http.StatusServiceUnavailable, "secrets_unconfigured",
			"secret storage is not configured on this server")
		return
	}

	var body credentialBody
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "expected {username, password}")
		return
	}

	username := strings.TrimSpace(body.Username)
	if username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_credential", "username and password are required")
		return
	}

	oltID := r.PathValue("id")
	if _, err := s.store.Olt(r.Context(), oltID); err != nil {
		s.handleStoreError(w, err)
		return
	}

	user, _ := userFrom(r.Context())
	if err := s.secrets.Set(r.Context(), oltID, username, body.Password, user.ID); err != nil {
		s.log.Error("store credential", "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "could not store the credential")
		return
	}

	s.auditActor(r, "olt.credential.set", oltID, username)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getCredentialMeta(w http.ResponseWriter, r *http.Request) {
	if s.secrets == nil {
		writeError(w, http.StatusServiceUnavailable, "secrets_unconfigured",
			"secret storage is not configured on this server")
		return
	}

	meta, err := s.secrets.Meta(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) deleteCredential(w http.ResponseWriter, r *http.Request) {
	if s.secrets == nil {
		writeError(w, http.StatusServiceUnavailable, "secrets_unconfigured",
			"secret storage is not configured on this server")
		return
	}

	oltID := r.PathValue("id")
	if err := s.secrets.Delete(r.Context(), oltID); err != nil {
		s.handleStoreError(w, err)
		return
	}
	s.auditActor(r, "olt.credential.delete", oltID, "")
	w.WriteHeader(http.StatusNoContent)
}

// auditActor records who changed what. A failed audit write must not fail the
// operation — the change already happened — but it is logged loudly.
func (s *Server) auditActor(r *http.Request, action, target, detail string) {
	if s.audit == nil {
		return
	}
	actor := "anonymous"
	if user, ok := userFrom(r.Context()); ok {
		actor = user.Username
	}
	if err := s.audit.RecordAudit(r.Context(), actor, action, target, detail); err != nil {
		s.log.Error("audit", "action", action, "error", err)
	}
}

// deleteOlt removes a device and everything that cascades from it: ports, ONUs,
// measurements, events, incidents. Irreversible, so it is audited with the
// device name before the row disappears.
func (s *Server) deleteOlt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	olt, err := s.store.Olt(r.Context(), id)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	if err := s.devices.DeleteOlt(r.Context(), id); err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.auditActor(r, "olt.delete", id, olt.Name)
	w.WriteHeader(http.StatusNoContent)
}
