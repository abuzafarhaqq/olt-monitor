package api

import (
	"net/http"
	"time"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collecting"
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
	plan, runID, err := s.collector.Run(r.Context(), device)
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
		"runId":  runID,
		"plan":   plan,
		"stored": true,
		"tookMs": time.Since(started).Milliseconds(),
	})
}
