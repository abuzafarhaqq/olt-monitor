package api_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abuzafarhaqq/olt-monitor-backend/internal/api"
	"github.com/abuzafarhaqq/olt-monitor-backend/internal/domain"
	"github.com/abuzafarhaqq/olt-monitor-backend/internal/store/memory"
)

func newServer(t *testing.T) http.Handler {
	t.Helper()
	fixed := time.Date(2026, 11, 6, 14, 0, 0, 0, time.UTC)
	st := memory.New(memory.WithClock(func() time.Time { return fixed }))
	return api.NewServer(st, nil).WithHeartbeat(20 * time.Millisecond).Routes()
}

func get(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestOverviewShape(t *testing.T) {
	rec := get(t, newServer(t), "/api/v1/dashboard/overview")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var overview domain.Overview
	if err := json.Unmarshal(rec.Body.Bytes(), &overview); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(overview.Olts) == 0 || len(overview.Onus) == 0 {
		t.Fatal("overview is empty")
	}

	// Every ONU must land in exactly one bucket; unknown may never be folded
	// into offline.
	online, offline, unknown := 0, 0, 0
	for _, onu := range overview.Onus {
		switch onu.Status {
		case domain.OnuOnline:
			online++
		case domain.OnuOffline:
			offline++
		case domain.OnuUnknown:
			unknown++
		default:
			t.Fatalf("unexpected status %q", onu.Status)
		}
	}
	if online+offline+unknown != len(overview.Onus) {
		t.Fatalf("buckets %d+%d+%d != %d", online, offline, unknown, len(overview.Onus))
	}
	if len(overview.AvailabilityTrend) == 0 {
		t.Fatal("availability trend is empty")
	}
}

// The central rule: an unreachable OLT must not turn its ONUs into "offline".
func TestFailedCollectionKeepsOnusUnknown(t *testing.T) {
	rec := get(t, newServer(t), "/api/v1/dashboard/overview")
	var overview domain.Overview
	if err := json.Unmarshal(rec.Body.Bytes(), &overview); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var olt *domain.Olt
	for i := range overview.Olts {
		if overview.Olts[i].ID == "olt-2" {
			olt = &overview.Olts[i]
		}
	}
	if olt == nil {
		t.Fatal("fixture is missing olt-2")
	}
	if olt.CollectionStatus != domain.CollectionFailed {
		t.Fatalf("olt-2 collection = %q, want failed", olt.CollectionStatus)
	}

	seen := 0
	for _, onu := range overview.Onus {
		if onu.OltID != "olt-2" {
			continue
		}
		seen++
		if onu.Status == domain.OnuOffline {
			t.Fatalf("ONU %s was marked offline by a failed collection", onu.ID)
		}
		if onu.Status != domain.OnuUnknown {
			t.Fatalf("ONU %s status = %q, want unknown", onu.ID, onu.Status)
		}
	}
	if seen == 0 {
		t.Fatal("no ONUs on the failed OLT")
	}
}

func TestDuplicateEventsArePreserved(t *testing.T) {
	rec := get(t, newServer(t), "/api/v1/events")
	var events []domain.Event
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("decode: %v", err)
	}

	ids := map[string]bool{}
	for _, event := range events {
		ids[event.ID] = true
	}
	if !ids["evt-1"] || !ids["evt-2"] {
		t.Fatal("identical-looking events must both be returned, each with its own evidence")
	}
}

func TestNotFound(t *testing.T) {
	rec := get(t, newServer(t), "/api/v1/onus/does-not-exist")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Fatalf("error code = %q, want not_found", body.Error.Code)
	}
}

func TestOpticalHistoryKeepsGaps(t *testing.T) {
	rec := get(t, newServer(t), "/api/v1/onus/olt-1-pon1-onu2/optical")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var points []domain.OpticalPoint
	if err := json.Unmarshal(rec.Body.Bytes(), &points); err != nil {
		t.Fatalf("decode: %v", err)
	}
	missing := 0
	for _, point := range points {
		if point.RxPowerDbm == nil {
			missing++
		}
	}
	if missing == 0 {
		t.Fatal("a missing sample must stay a nil gap, not become 0 dBm")
	}
}

func TestStreamSendsReadyFrame(t *testing.T) {
	handler := newServer(t)
	server := httptest.NewServer(handler)
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/v1/stream", nil)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content-type = %q", got)
	}

	reader := bufio.NewReader(resp.Body)
	first, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if first != "event: ready\n" {
		t.Fatalf("first line = %q, want ready frame", first)
	}
}
