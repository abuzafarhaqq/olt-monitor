package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/domain"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store/postgres"
	"github.com/abuzafarhaqq/olt-monitor/db"
)

// These tests need a real database:
//
//	TEST_DATABASE_URL=postgres://user:pass@localhost:5432/oltmon_test go test ./...
//
// They are skipped, not faked, when it is absent.
func openStore(t *testing.T) *postgres.Store {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	st, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(st.Close)

	if err := postgres.Migrate(ctx, st.Pool(), db.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := postgres.Seed(ctx, st.Pool()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return st
}

func TestOverviewBuckets(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()

	overview, err := st.Overview(ctx)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}

	if len(overview.Olts) != 2 {
		t.Fatalf("olts = %d, want 2", len(overview.Olts))
	}
	if len(overview.Pons) != 3 {
		t.Fatalf("pons = %d, want 3", len(overview.Pons))
	}

	online, offline, unknown := 0, 0, 0
	for _, onu := range overview.Onus {
		switch onu.Status {
		case domain.OnuOnline:
			online++
		case domain.OnuOffline:
			offline++
		case domain.OnuUnknown:
			unknown++
		}
	}
	if online+offline+unknown != len(overview.Onus) {
		t.Fatalf("buckets do not add up: %d+%d+%d vs %d", online, offline, unknown, len(overview.Onus))
	}
	if unknown == 0 || offline == 0 {
		t.Fatalf("fixture must exercise both buckets: offline=%d unknown=%d", offline, unknown)
	}
}

// The rule the whole product hangs on.
func TestFailedCollectionKeepsOnusUnknown(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()

	overview, err := st.Overview(ctx)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}

	var failedOlt string
	for _, olt := range overview.Olts {
		if olt.CollectionStatus == domain.CollectionFailed {
			failedOlt = olt.ID
		}
	}
	if failedOlt == "" {
		t.Fatal("fixture has no failed OLT")
	}

	seen := 0
	for _, onu := range overview.Onus {
		if onu.OltID != failedOlt {
			continue
		}
		seen++
		if onu.Status == domain.OnuOffline {
			t.Fatalf("ONU %s marked offline by a failed collection", onu.ID)
		}
	}
	if seen == 0 {
		t.Fatal("no ONUs on the failed OLT")
	}
}

func TestPonCountsAreDerived(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()

	pons, err := st.PonsByOlt(ctx, "olt-1")
	if err != nil {
		t.Fatalf("pons: %v", err)
	}
	for _, pon := range pons {
		counts := pon.Counts
		if counts.Total != counts.Online+counts.Offline+counts.Unknown {
			t.Fatalf("pon %s: total %d != online %d + offline %d + unknown %d",
				pon.ID, counts.Total, counts.Online, counts.Offline, counts.Unknown)
		}
	}
}

// Low optical must come from the threshold table, not from a Go constant.
func TestLowOpticalUsesCalibratedThreshold(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()

	pons, err := st.PonsByOlt(ctx, "olt-1")
	if err != nil {
		t.Fatalf("pons: %v", err)
	}

	total := 0
	for _, pon := range pons {
		total += pon.LowOpticalCount
	}
	// -31.4 dBm is below VSOL/EPON critical (-27); -18.4 is well above it.
	if total != 1 {
		t.Fatalf("low optical count = %d, want 1", total)
	}
}

func TestUnknownOnuIsNotFound(t *testing.T) {
	st := openStore(t)
	if _, err := st.Onu(context.Background(), "nope"); err == nil {
		t.Fatal("expected an error for a missing ONU")
	} else if err != store.ErrNotFound {
		t.Fatalf("error = %v, want store.ErrNotFound", err)
	}
}

func TestDuplicateEventsAndNilGaps(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()

	events, err := st.Events(ctx)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	seen := map[string]bool{}
	for _, event := range events {
		seen[event.ID] = true
	}
	if !seen["evt-1"] || !seen["evt-2"] {
		t.Fatal("both captures of the same event must survive")
	}

	points, err := st.OpticalHistory(ctx, "olt-1-pon1-onu2")
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("expected at least one sample")
	}
	if points[0].RxPowerDbm == nil {
		t.Fatal("a stored reading must not come back as nil")
	}

	empty, err := st.OpticalHistory(ctx, "olt-1-pon2-onu3")
	if err != nil {
		t.Fatalf("history for ONU without optics: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("ONU with no optical command returned %d samples", len(empty))
	}
}

func TestIncidentDetail(t *testing.T) {
	st := openStore(t)
	ctx := context.Background()

	incident, err := st.Incident(ctx, "INC-2411-007")
	if err != nil {
		t.Fatalf("incident: %v", err)
	}
	if len(incident.Notes) != 1 {
		t.Fatalf("notes = %d, want 1", len(incident.Notes))
	}
	if len(incident.EvidenceEventIDs) != 2 {
		t.Fatalf("evidence = %d, want 2", len(incident.EvidenceEventIDs))
	}
	if incident.Hypothesis == nil || *incident.Hypothesis == "" {
		t.Fatal("hypothesis must be stored as text, not inferred")
	}
}
