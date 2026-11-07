package collector_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/collector"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store/postgres"
	"github.com/abuzafarhaqq/olt-monitor/db"
)

// The ingest path is tested with the captured outputs, not a device: these are
// the same rows the parser produces from the real transcripts.
func openPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping ingest tests")
	}

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	ctx := context.Background()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// Self-sufficient: apply migrations and make sure a device exists for the
	// foreign keys. Seeding is deliberately not done here — it truncates, and
	// test packages run in parallel against the same database.
	if err := postgres.Migrate(ctx, pool, db.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO olts (id, name, vendor, model, technology, management_address, management_port, connection, collection_profile)
		VALUES ('olt-1', 'EPON-OLT-01', 'VSOL', 'EPON-OLT', 'EPON', '192.0.2.11', 22, 'ssh', 'standard-5m')
		ON CONFLICT (id) DO NOTHING`); err != nil {
		t.Fatalf("ensure olt: %v", err)
	}
	return pool
}

func TestIngestMergesStatusAndIdentity(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	// Start from a clean device.
	if _, err := pool.Exec(ctx, `DELETE FROM optical_measurements WHERE onu_id LIKE 'olt-1%'`); err != nil {
		t.Fatalf("clean measurements: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM onus WHERE olt_id = 'olt-1'`); err != nil {
		t.Fatalf("clean onus: %v", err)
	}

	at := time.Date(2026, 11, 7, 21, 0, 0, 0, time.UTC)
	run := collector.Run{
		OltID: "olt-1",
		At:    at,
		Jobs: []collector.Job{
			{
				ResultID: "onu-status",
				Line:     "show onu status pon 1",
				Raw:      "ONU-ID Status\nEPON0/1:1 online\nEPON0/1:2 offline\nEPON0/1:3 online\n",
				Rows: []collector.OnuStatusRow{
					{Pon: "0/1", OnuID: 1, MAC: "02:00:00:00:00:01", Online: true},
					{Pon: "0/1", OnuID: 2, MAC: "02:00:00:00:00:02", Online: false},
					{Pon: "0/1", OnuID: 3, MAC: "02:00:00:00:00:03", Online: true},
				},
			},
			{
				// ONU 2 is deliberately absent: it is an unprovisioned entry.
				ResultID: "onu-basic-info",
				Line:     "show onu basic-info all",
				Raw:      "EPON0/1:1 MONU D301\nEPON0/1:3 HWTC XPON\n",
				Rows: []collector.OnuIdentityRow{
					{Pon: "0/1", OnuID: 1, Vendor: "MONU", Model: "D301", HardwareVer: "V8.0", SoftwareVer: "V6.0.1P1T2", Type: "SFU", InterfaceType: "1GE"},
					{Pon: "0/1", OnuID: 3, Vendor: "HWTC", Model: "XPON", HardwareVer: "V9.0", SoftwareVer: "V9.0.51P1T11", Type: "SFU", InterfaceType: "1GE"},
				},
			},
			{
				ResultID: "opm-diag",
				Line:     "show onu opm-diag pon 1",
				Raw:      "EPON0/1:1 58.88 3.22 12.69 3.02 -16.60\nEPON0/1:3 51.30 3.33 9.35 2.83 -17.01\n",
				Rows: []collector.OpticalReading{
					{Pon: "0/1", OnuID: 1, TemperatureC: 58.88, SupplyVoltageV: 3.22, TxBiasMa: 12.69, TxDbm: 3.02, RxDbm: -16.60},
					{Pon: "0/1", OnuID: 3, TemperatureC: 51.30, SupplyVoltageV: 3.33, TxBiasMa: 9.35, TxDbm: 2.83, RxDbm: -17.01},
				},
			},
		},
	}

	ing := collector.Ingest{Pool: pool, ParserVersion: "test@0.1"}
	runID, err := ing.Save(ctx, run)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if runID == 0 {
		t.Fatal("no run id returned")
	}

	var evidence int
	if err := pool.QueryRow(ctx,
		`SELECT count(*)::int FROM raw_evidence WHERE collection_run_id = $1`, runID).Scan(&evidence); err != nil {
		t.Fatalf("evidence count: %v", err)
	}
	if evidence != 3 {
		t.Fatalf("evidence rows = %d, want 3", evidence)
	}

	var onus, provisioned, statusOnly int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)::int,
		       count(*) FILTER (WHERE identity_source = 'basic-info')::int,
		       count(*) FILTER (WHERE identity_source = 'status-only')::int
		  FROM onus WHERE olt_id = 'olt-1'`).Scan(&onus, &provisioned, &statusOnly); err != nil {
		t.Fatalf("onu counts: %v", err)
	}
	if onus != 3 || provisioned != 2 || statusOnly != 1 {
		t.Fatalf("onus=%d provisioned=%d status-only=%d, want 3/2/1", onus, provisioned, statusOnly)
	}

	// The provisioned ONU keeps the status the device reported.
	var status, vendor, model string
	if err := pool.QueryRow(ctx, `
		SELECT status::text, vendor_id, model FROM onus WHERE id = 'olt-1-pon0-1-onu1'`).
		Scan(&status, &vendor, &model); err != nil {
		t.Fatalf("read onu 1: %v", err)
	}
	if status != "online" || vendor != "MONU" || model != "D301" {
		t.Fatalf("onu 1 = %s %s/%s", status, vendor, model)
	}

	var measurements int
	if err := pool.QueryRow(ctx,
		`SELECT count(*)::int FROM optical_measurements WHERE collection_run_id = $1`, runID).Scan(&measurements); err != nil {
		t.Fatalf("measurement count: %v", err)
	}
	if measurements != 2 {
		t.Fatalf("measurements = %d, want 2", measurements)
	}
}

// A run where every command failed must still record evidence, and must not
// touch ONU state.
func TestIngestFailedRunChangesNothing(t *testing.T) {
	pool := openPool(t)
	ctx := context.Background()

	var before int
	if err := pool.QueryRow(ctx, `SELECT count(*)::int FROM onus WHERE olt_id = 'olt-1'`).Scan(&before); err != nil {
		t.Fatalf("count: %v", err)
	}

	ing := collector.Ingest{Pool: pool, ParserVersion: "test@0.1"}
	runID, err := ing.Save(ctx, collector.Run{
		OltID: "olt-1", At: time.Now().UTC(), Failed: true, Reason: "timeout",
		Jobs: []collector.Job{{
			ResultID: "onu-status", Line: "show onu status pon 1",
			Raw: "timed out", Err: context.DeadlineExceeded,
		}},
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	var after, evidence int
	_ = pool.QueryRow(ctx, `SELECT count(*)::int FROM onus WHERE olt_id = 'olt-1'`).Scan(&after)
	_ = pool.QueryRow(ctx, `SELECT count(*)::int FROM raw_evidence WHERE collection_run_id = $1`, runID).Scan(&evidence)

	if after != before {
		t.Fatalf("a failed run changed ONU rows: %d -> %d", before, after)
	}
	if evidence != 1 {
		t.Fatalf("a failed run lost its evidence: %d rows", evidence)
	}
}
