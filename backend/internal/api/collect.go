package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collecting"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/live"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store/postgres"
)

// WithCollector enables "refresh this OLT" from the UI.
func (s *Server) WithCollector(service *collecting.Service) *Server {
	s.collector = service
	return s
}

// WithHub enables the live terminal stream.
func (s *Server) WithHub(hub *live.Hub) *Server {
	s.hub = hub
	return s
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
// order, before anything is dialled.
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

// collectOlt starts a run and returns 202 with the run id.
//
// The work runs in a goroutine with its own context: a browser closing the tab,
// or an HTTP client with a short timeout, must not abort a collection halfway
// and leave the row in 'running' with no evidence.
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

	runID, err := s.collector.Start(r.Context(), device)
	if err != nil {
		if errors.Is(err, collecting.ErrAlreadyRunning) {
			writeError(w, http.StatusConflict, "already_running", err.Error())
			return
		}
		s.log.Warn("collection could not start", "olt", device.ID, "error", err)
		writeError(w, http.StatusBadGateway, "collection_failed", err.Error())
		return
	}

	s.auditActor(r, "olt.collect", device.ID, device.Name)
	w.Header().Set("Location", fmt.Sprintf("/api/v1/collection/runs/%d/stream", runID))
	writeJSON(w, http.StatusAccepted, map[string]any{"runId": runID, "stream": "sse"})
}

// runStream is the live terminal: SSE, view-only, cookie-authenticated.
//
// SSE rather than a WebSocket on purpose — the terminal never sends input, so
// no keystroke can bypass the command allow-list.
func (s *Server) runStream(w http.ResponseWriter, r *http.Request) {
	if s.hub == nil {
		writeError(w, http.StatusServiceUnavailable, "stream_disabled", "live streaming is not configured")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "run id must be a number")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported", "response writer cannot stream")
		return
	}

	after := int64(0)
	if header := r.Header.Get("Last-Event-ID"); header != "" {
		if parsed, err := strconv.ParseInt(header, 10, 64); err == nil {
			after = parsed
		}
	}
	if query := r.URL.Query().Get("from"); query != "" {
		if parsed, err := strconv.ParseInt(query, 10, 64); err == nil && parsed > after {
			after = parsed
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	write := func(ev live.Event) bool {
		payload, err := json.Marshal(ev)
		if err != nil {
			return true
		}
		if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.Seq, ev.Kind, payload); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	// If the ring has already forgotten what the client is missing, say so
	// instead of starting mid-table as though nothing were lost.
	if first, ok := s.hub.FirstSeq(id); ok && first > after+1 {
		fmt.Fprintf(w, "event: truncated\ndata: {\"kind\":\"truncated\",\"data\":\"%d\"}\n\n", first-after-1)
		flusher.Flush()
	}

	for _, ev := range s.hub.History(id, after) {
		if !write(ev) {
			return
		}
	}

	events, cancel := s.hub.Subscribe(id)
	defer cancel()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-events:
			if !ok {
				// The hub closes a subscriber that stops reading, rather than
				// dropping bytes mid-escape-sequence. Tell the client how much
				// it missed and let EventSource reconnect with Last-Event-ID.
				if dropped := s.hub.Dropped(id); dropped > 0 {
					fmt.Fprintf(w, "event: truncated\ndata: {\"kind\":\"truncated\",\"data\":\"%d\"}\n\n", dropped)
					flusher.Flush()
				}
				return
			}
			if !write(ev) {
				return
			}
		}
	}
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
