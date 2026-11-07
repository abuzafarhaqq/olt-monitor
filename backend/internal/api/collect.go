package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collecting"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store/postgres"
)

// WithCollector enables "refresh this OLT" from the UI. Without it the routes
// answer 503 rather than pretending a device was contacted.
func (s *Server) WithCollector(service *collecting.Service) *Server {
	s.collector = service
	return s
}

func (s *Server) deviceFor(r *http.Request) (collecting.Device, error) {
	olt, err := s.store.Olt(r.Context(), r.PathValue("id"))
	if err != nil {
		return collecting.Device{}, err
	}
	return collecting.Device{
		ID: olt.ID, Name: olt.Name, Vendor: olt.Vendor, Model: olt.Model,
		Technology: olt.Technology, ManagementAddress: olt.ManagementAddress,
		ManagementPort: 22, Connection: olt.Connection,
	}, nil
}

// collectionPlan is what the confirmation dialog shows: the exact commands, in
// order, before anything is dialled. No secret, no side effect.
func (s *Server) collectionPlan(w http.ResponseWriter, r *http.Request) {
	if s.collector == nil {
		writeError(w, http.StatusServiceUnavailable, "collector_disabled", "collection is not configured")
		return
	}

	device, err := s.deviceFor(r)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	plan, err := s.collector.BuildPlan(device)
	if err != nil {
		writeError(w, http.StatusBadRequest, "no_adapter", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// collectOlt runs the plan and stores the result. It is synchronous: the HTTP
// request is the job. A 90-second SSH session inside a request is acceptable for
// four devices and one operator; it becomes a queue when there are fifty.
func (s *Server) collectOlt(w http.ResponseWriter, r *http.Request) {
	if s.collector == nil {
		writeError(w, http.StatusServiceUnavailable, "collector_disabled", "collection is not configured")
		return
	}

	device, err := s.deviceFor(r)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}

	started := time.Now()
	plan, runID, results, err := s.collector.Run(r.Context(), device)
	if err != nil {
		// A device that cannot be reached is not a server fault: 502 with the
		// reason the operator needs.
		s.log.Warn("collection failed", "olt", device.ID, "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":    map[string]string{"code": "collection_failed", "message": err.Error()},
			"plan":     plan,
			"waitedMs": time.Since(started).Milliseconds(),
		})
		return
	}

	s.auditActor(r, "olt.collect", device.ID, device.Name)
	writeJSON(w, http.StatusOK, map[string]any{
		"runId":   runID,
		"plan":    plan,
		"results": results,
		"stored":  true,
		"tookMs":  time.Since(started).Milliseconds(),
	})
}

// WithLogs enables the collection log page.
func (s *Server) WithLogs(logs LogStore) *Server {
	s.logs = logs
	return s
}

// LogStore is the read side of the collection log.
type LogStore interface {
	ListRuns(ctx context.Context, limit int) ([]postgres.RunSummary, error)
	RunEvidence(ctx context.Context, runID int64) ([]postgres.EvidenceRow, error)
	RunExists(ctx context.Context, runID int64) error
}

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	if s.logs == nil {
		writeError(w, http.StatusServiceUnavailable, "logs_disabled", "the collection log is not configured")
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}

	runs, err := s.logs.ListRuns(r.Context(), limit)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) runDetail(w http.ResponseWriter, r *http.Request) {
	if s.logs == nil {
		writeError(w, http.StatusServiceUnavailable, "logs_disabled", "the collection log is not configured")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "run id must be a number")
		return
	}
	if err := s.logs.RunExists(r.Context(), id); err != nil {
		s.handleStoreError(w, err)
		return
	}

	evidence, err := s.logs.RunEvidence(r.Context(), id)
	if err != nil {
		s.handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "evidence": evidence})
}
