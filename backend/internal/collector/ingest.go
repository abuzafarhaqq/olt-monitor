package collector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Ingest writes one collection run to the database, in a single transaction.
//
// The rule this file exists to enforce: a run either lands completely or not at
// all, and a failed command writes evidence without inventing ONU state. Nothing
// here updates an ONU's status from a run that did not answer.
type Ingest struct {
	Pool *pgxpool.Pool
	// ParserVersion is stored beside every piece of raw evidence, so a later
	// parser fix can coexist with rows written by an older one.
	ParserVersion string
}

// Job is one parsed command result together with the rows it produced.
type Job struct {
	ResultID string
	Line     string
	Raw      string
	Rows     any
	Err      error
}

// Run is everything one collection cycle produced.
type Run struct {
	OltID  string
	Jobs   []Job
	At     time.Time
	Failed bool
	Reason string
}

// Save writes the run. Commands that failed are recorded as evidence with their
// error, and contribute nothing to ONU state.
func (in Ingest) Save(ctx context.Context, run Run) (int64, error) {
	tx, err := in.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result := "successful"
	if run.Failed {
		result = "failed"
	}

	var runID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO collection_runs (olt_id, job_type, started_at, finished_at, result, error_message)
		VALUES ($1, 'full', $2, now(), $3::job_result, NULLIF($4, ''))
		RETURNING id`,
		run.OltID, run.At, result, run.Reason).Scan(&runID); err != nil {
		return 0, fmt.Errorf("insert run: %w", err)
	}

	version := in.ParserVersion
	if version == "" {
		version = "unknown"
	}

	for _, job := range run.Jobs {
		// Evidence is written even when parsing failed: the raw text is the
		// only thing that can explain why.
		completeness := 1.0
		if job.Err != nil {
			completeness = 0
		}
		evidenceID := fmt.Sprintf("%s-%s-%s", run.OltID, job.ResultID, run.At.UTC().Format("20060102T150405"))
		if _, err := tx.Exec(ctx, `
			INSERT INTO raw_evidence (id, collection_run_id, olt_id, command, captured_at, parser_version, completeness, body)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO NOTHING`,
			evidenceID, runID, run.OltID, job.Line, run.At, version, completeness, job.Raw); err != nil {
			return 0, fmt.Errorf("insert evidence: %w", err)
		}

		if job.Err != nil || job.Rows == nil {
			continue
		}

		switch rows := job.Rows.(type) {
		case []OnuStatusRow:
			if err := upsertStatus(ctx, tx, run.OltID, run.At, rows); err != nil {
				return 0, err
			}
		case []OnuIdentityRow:
			if err := upsertIdentity(ctx, tx, run.OltID, rows); err != nil {
				return 0, err
			}
		case []OpticalReading:
			if err := insertOptical(ctx, tx, run.OltID, runID, run.At, evidenceID, rows); err != nil {
				return 0, err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return runID, nil
}

// ponID and onuID are derived, not random: the same ONU must get the same row
// on every run, or history is worthless.
func ponID(oltID, pon string) string {
	return fmt.Sprintf("%s-pon%s", oltID, strings.ReplaceAll(pon, "/", "-"))
}

func onuID(oltID, pon string, number int) string {
	return fmt.Sprintf("%s-onu%d", ponID(oltID, pon), number)
}

func ensurePon(ctx context.Context, tx pgx.Tx, oltID, pon string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO pon_ports (id, olt_id, name)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO NOTHING`,
		ponID(oltID, pon), oltID, "EPON"+pon)
	if err != nil {
		return fmt.Errorf("ensure pon %s: %w", pon, err)
	}
	return nil
}

// OnuStatusRow is one ONU as `show onu status` reported it.
type OnuStatusRow struct {
	Pon      string
	OnuID    int
	MAC      string
	Online   bool
	Distance *int
}

// upsertStatus records what the device said. An ONU that has never appeared in
// basic-info keeps identity_source 'status-only': the platform must be able to
// tell "unprovisioned entry" from "provisioned ONU that is down".
func upsertStatus(ctx context.Context, tx pgx.Tx, oltID string, at time.Time, rows []OnuStatusRow) error {
	for _, row := range rows {
		if err := ensurePon(ctx, tx, oltID, row.Pon); err != nil {
			return err
		}

		status := "offline"
		if row.Online {
			status = "online"
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO onus (id, olt_id, pon_id, onu_number, mac, status, status_observed_at, identity_source)
			VALUES ($1, $2, $3, $4, $5::macaddr, $6::onu_status, $7, 'status-only')
			ON CONFLICT (id) DO UPDATE
			   SET status = EXCLUDED.status,
			       status_observed_at = EXCLUDED.status_observed_at,
			       mac = EXCLUDED.mac,
			       -- identity wins: a status row must not demote a known ONU
			       identity_source = CASE WHEN onus.identity_source = 'basic-info'
			                              THEN onus.identity_source ELSE 'status-only' END`,
			onuID(oltID, row.Pon, row.OnuID), oltID, ponID(oltID, row.Pon), row.OnuID, row.MAC, status, at); err != nil {
			return fmt.Errorf("upsert onu %s:%d: %w", row.Pon, row.OnuID, err)
		}
	}
	return nil
}

// OnuIdentityRow is one line of `show onu basic-info`.
type OnuIdentityRow struct {
	Pon           string
	OnuID         int
	Vendor        string
	Model         string
	HardwareVer   string
	SoftwareVer   string
	Type          string
	InterfaceType string
}

func upsertIdentity(ctx context.Context, tx pgx.Tx, oltID string, rows []OnuIdentityRow) error {
	for _, row := range rows {
		if err := ensurePon(ctx, tx, oltID, row.Pon); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO onus (id, olt_id, pon_id, onu_number, mac, status,
			                  vendor_id, model, hardware_ver, software_ver,
			                  onu_type, interface_type, identity_source, threshold_model)
			VALUES ($1, $2, $3, $4, '00:00:00:00:00:00'::macaddr, 'unknown',
			        $5, $6, $7, $8, $9, $10, 'basic-info', $11)
			ON CONFLICT (id) DO UPDATE
			   SET vendor_id = EXCLUDED.vendor_id,
			       model = EXCLUDED.model,
			       hardware_ver = EXCLUDED.hardware_ver,
			       software_ver = EXCLUDED.software_ver,
			       onu_type = EXCLUDED.onu_type,
			       interface_type = EXCLUDED.interface_type,
			       identity_source = 'basic-info',
			       threshold_model = EXCLUDED.threshold_model`,
			onuID(oltID, row.Pon, row.OnuID), oltID, ponID(oltID, row.Pon), row.OnuID,
			row.Vendor, row.Model, row.HardwareVer, row.SoftwareVer,
			row.Type, row.InterfaceType, row.Vendor+"/"+row.Model); err != nil {
			return fmt.Errorf("upsert identity %s:%d: %w", row.Pon, row.OnuID, err)
		}
	}
	return nil
}

// OpticalReading is one line of `show onu opm-diag`.
type OpticalReading struct {
	Pon            string
	OnuID          int
	RxDbm          float64
	TxDbm          float64
	TemperatureC   float64
	SupplyVoltageV float64
	TxBiasMa       float64
}

func insertOptical(ctx context.Context, tx pgx.Tx, oltID string, runID int64, at time.Time, evidenceID string, rows []OpticalReading) error {
	for _, row := range rows {
		// A measurement without its ONU is not usable; skip rather than invent.
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM onus WHERE id = $1)`,
			onuID(oltID, row.Pon, row.OnuID)).Scan(&exists); err != nil {
			return fmt.Errorf("check onu: %w", err)
		}
		if !exists {
			continue
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO optical_measurements
			    (onu_id, collection_run_id, rx_power_dbm, tx_power_dbm, temperature_c,
			     supply_voltage_v, tx_bias_ma, observed_at, timestamp_quality, evidence_ref)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'untrusted', $9)`,
			onuID(oltID, row.Pon, row.OnuID), runID, row.RxDbm, row.TxDbm,
			row.TemperatureC, row.SupplyVoltageV, row.TxBiasMa, at, evidenceID); err != nil {
			return fmt.Errorf("insert optical %s:%d: %w", row.Pon, row.OnuID, err)
		}
	}
	return nil
}

// NewIngest opens a pool for a one-shot run.
func NewIngest(ctx context.Context, dsn, parserVersion string) (*Ingest, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Ingest{Pool: pool, ParserVersion: parserVersion}, nil
}

func (in *Ingest) Close() {
	if in.Pool != nil {
		in.Pool.Close()
	}
}

// StartRun inserts a run in the 'running' state and returns its id. The API
// needs the id before the collection finishes, so the row cannot wait for the
// result.
func (in Ingest) StartRun(ctx context.Context, oltID string) (int64, error) {
	var runID int64
	if err := in.Pool.QueryRow(ctx, `
		INSERT INTO collection_runs (olt_id, job_type, started_at, result, live)
		VALUES ($1, 'full', now(), 'running', TRUE)
		RETURNING id`, oltID).Scan(&runID); err != nil {
		return 0, fmt.Errorf("start run: %w", err)
	}
	return runID, nil
}

// Append writes evidence and rows into an existing run.
// appendSerial writes every row in one transaction. Fine for a CLI run of a
// few hundred ONUs; the estate path uses Append (ingest_batch.go).
func (in Ingest) appendSerial(ctx context.Context, runID int64, oltID string, at time.Time, jobs []Job) error {
	tx, err := in.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	version := in.ParserVersion
	if version == "" {
		version = "unknown"
	}

	for _, job := range jobs {
		completeness := 1.0
		if job.Err != nil {
			completeness = 0
		}
		evidenceID := fmt.Sprintf("run%d-%s", runID, job.ResultID)
		if _, err := tx.Exec(ctx, `
			INSERT INTO raw_evidence (id, collection_run_id, olt_id, command, captured_at, parser_version, completeness, body)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO NOTHING`,
			evidenceID, runID, oltID, job.Line, at, version, completeness, job.Raw); err != nil {
			return fmt.Errorf("insert evidence: %w", err)
		}
		if job.Err != nil || job.Rows == nil {
			continue
		}

		switch rows := job.Rows.(type) {
		case []OnuStatusRow:
			if err := upsertStatus(ctx, tx, oltID, at, rows); err != nil {
				return err
			}
		case []OnuIdentityRow:
			if err := upsertIdentity(ctx, tx, oltID, rows); err != nil {
				return err
			}
		case []OpticalReading:
			if err := insertOptical(ctx, tx, oltID, runID, at, evidenceID, rows); err != nil {
				return err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// FinishRun closes a run with its result.
func (in Ingest) FinishRun(ctx context.Context, runID int64, result, reason string, streamed int) error {
	if _, err := in.Pool.Exec(ctx, `
		UPDATE collection_runs
		   SET finished_at = now(), result = $2::job_result,
		       error_message = NULLIF($3, ''), streamed_events = $4
		 WHERE id = $1`, runID, result, reason, streamed); err != nil {
		return fmt.Errorf("finish run: %w", err)
	}
	return nil
}
