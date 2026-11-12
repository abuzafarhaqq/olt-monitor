// Package api is the HTTP surface. It holds no business rules of its own: it
// validates identifiers, calls the store and serialises the result. The rules
// live in the data the store returns (an ONU is never offline because a
// collection failed) and in the schema that enforces them.
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/auth"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collecting"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/live"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/secrets"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store"
)

type Server struct {
	build     string
	store     store.Store
	auth      store.AuthStore
	audit     store.AuditStore
	devices   store.DeviceStore
	users     store.UserStore
	collector *collecting.Service
	hub       *live.Hub
	logs      LogStore
	secrets   *secrets.Service
	log       *slog.Logger
	heartbeat time.Duration
}

func NewServer(s store.Store, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{store: s, log: logger, heartbeat: 15 * time.Second}
}

// WithHeartbeat shortens the SSE interval so tests do not wait 15 seconds.
func (s *Server) WithHeartbeat(d time.Duration) *Server {
	s.heartbeat = d
	return s
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", s.health)
	mux.HandleFunc("GET /api/v1/dashboard/overview", s.guardRead(s.overview))
	mux.HandleFunc("GET /api/v1/olts", s.guardRead(s.olts))
	mux.HandleFunc("GET /api/v1/olts/{id}", s.guardRead(s.olt))
	mux.HandleFunc("GET /api/v1/olts/{id}/pons", s.guardRead(s.pons))
	mux.HandleFunc("GET /api/v1/pons/{id}/onus", s.guardRead(s.onus))
	mux.HandleFunc("GET /api/v1/onus/{id}", s.guardRead(s.onu))
	mux.HandleFunc("GET /api/v1/onus/{id}/optical", s.guardRead(s.opticalHistory))
	mux.HandleFunc("GET /api/v1/events", s.guardRead(s.events))
	mux.HandleFunc("GET /api/v1/incidents", s.guardRead(s.incidents))
	mux.HandleFunc("GET /api/v1/incidents/{id}", s.guardRead(s.incident))
	mux.HandleFunc("GET /api/v1/collection/jobs", s.guardRead(s.collectionJobs))
	mux.HandleFunc("GET /api/v1/stream", s.stream)

	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.HandleFunc("GET /api/v1/auth/me", s.requireSession(s.me))
	mux.HandleFunc("POST /api/v1/auth/password-reset", s.passwordResetRequest)

	// Writes require a session and the right permission. The frontend hides the
	// buttons; this is what actually stops the call.
	mux.HandleFunc("POST /api/v1/olts", s.guard(auth.PermDeviceManage, s.createOlt))
	mux.HandleFunc("PATCH /api/v1/olts/{id}", s.guard(auth.PermDeviceManage, s.updateOlt))
	mux.HandleFunc("PUT /api/v1/olts/{id}/credential", s.guard(auth.PermDeviceManage, s.setCredential))
	mux.HandleFunc("GET /api/v1/olts/{id}/credential", s.guard(auth.PermDeviceRead, s.getCredentialMeta))
	mux.HandleFunc("DELETE /api/v1/olts/{id}/credential", s.guard(auth.PermDeviceManage, s.deleteCredential))
	mux.HandleFunc("DELETE /api/v1/olts/{id}", s.guard(auth.PermDeviceManage, s.deleteOlt))

	// Collection from the UI: the plan is readable by anyone who may read
	// diagnostics, running it needs the run permission.
	mux.HandleFunc("GET /api/v1/olts/{id}/collection-plan", s.guard(auth.PermDiagnosticsRead, s.collectionPlan))
	mux.HandleFunc("POST /api/v1/olts/{id}/collect", s.guard(auth.PermDiagnosticsRun, s.collectOlt))
	mux.HandleFunc("GET /api/v1/collection/runs/{id}/stream", s.guard(auth.PermDiagnosticsRead, s.runStream))

	mux.HandleFunc("GET /api/v1/collection/runs", s.guard(auth.PermDiagnosticsRead, s.listRuns))
	mux.HandleFunc("GET /api/v1/collection/runs/{id}", s.guard(auth.PermDiagnosticsRead, s.runDetail))

	mux.HandleFunc("GET /api/v1/users", s.guard(auth.PermUsersManage, s.listUsers))
	mux.HandleFunc("POST /api/v1/users", s.guard(auth.PermUsersManage, s.createUser))
	mux.HandleFunc("PATCH /api/v1/users/{id}", s.guard(auth.PermUsersManage, s.updateUser))
	mux.HandleFunc("PUT /api/v1/users/{id}/password", s.guard(auth.PermUsersManage, s.setUserPassword))
	mux.HandleFunc("DELETE /api/v1/users/{id}", s.guard(auth.PermUsersManage, s.deleteUser))

	return s.recoverer(s.logger(mux))
}

// --- middleware ---------------------------------------------------------

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Flush forwards to the underlying writer. Without this method the wrapper
// hides http.Flusher, the SSE handler's type assertion fails and every stream
// request returns 500 — a middleware bug that only a streaming test catches.
func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (s *Server) logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.log.Error("panic", "path", r.URL.Path, "panic", fmt.Sprint(recovered))
				writeError(w, http.StatusInternalServerError, "internal", "unexpected server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// --- responses ----------------------------------------------------------

type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		// The header is already sent; all we can do is log upstream.
		slog.Error("encode response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var body errorBody
	body.Error.Code = code
	body.Error.Message = message
	writeJSON(w, status, body)
}

// handleStoreError keeps the mapping in one place: a missing row is 404, and
// anything else is a 500 that does not leak internals to the client.
func (s *Server) handleStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "resource does not exist")
		return
	}
	s.log.Error("store error", "error", err)
	writeError(w, http.StatusInternalServerError, "internal", "storage error")
}

// --- handlers -----------------------------------------------------------

// WithBuild reports the binary's build time on /health, so a stale process is
// visible instead of being mistaken for a bug in the current code.
func (s *Server) WithBuild(build string) *Server {
	s.build = build
	return s
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"build":  s.build,
	})
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.Overview(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) olts(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.Olts(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) olt(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.Olt(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) pons(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.PonsByOlt(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) onus(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.OnusByPon(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) onu(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.Onu(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) opticalHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.Onu(r.Context(), id); err != nil {
		s.handleStoreError(w, err)
		return
	}
	points, err := s.store.OpticalHistory(r.Context(), id)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, points)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.Events(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) incidents(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.Incidents(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) incident(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.Incident(r.Context(), r.PathValue("id"))
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) collectionJobs(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.CollectionJobs(r.Context())
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// stream is the SSE endpoint. It sends an immediate ready frame, then a
// heartbeat. A client that reconnects must fetch /dashboard/overview again:
// SSE does not replay what was missed, and silently patching the gap would
// leave the UI holding stale state.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported", "response writer cannot stream")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	fmt.Fprintf(w, "event: ready\ndata: {\"at\":%q}\n\n", time.Now().UTC().Format(time.RFC3339))
	flusher.Flush()

	ticker := time.NewTicker(s.heartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case at := <-ticker.C:
			fmt.Fprintf(w, "event: heartbeat\ndata: {\"at\":%q}\n\n", at.UTC().Format(time.RFC3339))
			flusher.Flush()
		}
	}
}
