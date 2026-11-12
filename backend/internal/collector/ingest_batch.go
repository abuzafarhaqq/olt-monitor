package collector

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// BatchSize is how many statements are queued before a round trip. 400 keeps a
// 25,000-ONU estate to ~60 batches per PON instead of 25,000 round trips.
const BatchSize = 400

// ponWork is everything collected for one PON port, so it can be committed as a
// unit: a failure on port 3 must not discard ports 1 and 2.
type ponWork struct {
	status   []OnuStatusRow
	identity []OnuIdentityRow
	optical  []OpticalReading
}

// Append writes a run's rows in one transaction per PON, batching statements.
//
// Why per PON: a 25,000-ONU collection takes minutes, and one giant transaction
// holds locks for all of it and loses everything if the last port fails. Why
// batches: 25,000 individual executes is 25,000 round trips.
func (in Ingest) Append(ctx context.Context, runID int64, oltID string, at time.Time, jobs []Job) error {
	version := in.ParserVersion
	if version == "" {
		version = "unknown"
	}

	// Evidence first, in its own small transaction: it belongs to the run, not
	// to a port, and must survive even if every port's rows fail.
	if err := in.writeEvidence(ctx, runID, oltID, at, version, jobs); err != nil {
		return err
	}

	byPon := groupByPon(jobs)
	pons := make([]string, 0, len(byPon))
	for pon := range byPon {
		pons = append(pons, pon)
	}
	sort.Strings(pons)

	for _, pon := range pons {
		if err := in.writePon(ctx, runID, oltID, at, pon, byPon[pon]); err != nil {
			return fmt.Errorf("pon %s: %w", pon, err)
		}
	}
	return nil
}

func groupByPon(jobs []Job) map[string]*ponWork {
	byPon := map[string]*ponWork{}
	get := func(pon string) *ponWork {
		if _, ok := byPon[pon]; !ok {
			byPon[pon] = &ponWork{}
		}
		return byPon[pon]
	}

	for _, job := range jobs {
		if job.Err != nil || job.Rows == nil {
			continue
		}
		switch rows := job.Rows.(type) {
		case []OnuStatusRow:
			for _, row := range rows {
				work := get(row.Pon)
				work.status = append(work.status, row)
			}
		case []OnuIdentityRow:
			for _, row := range rows {
				work := get(row.Pon)
				work.identity = append(work.identity, row)
			}
		case []OpticalReading:
			for _, row := range rows {
				work := get(row.Pon)
				work.optical = append(work.optical, row)
			}
		}
	}
	return byPon
}

func (in Ingest) writeEvidence(ctx context.Context, runID int64, oltID string, at time.Time, version string, jobs []Job) error {
	tx, err := in.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin evidence: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	batch := &pgx.Batch{}
	for _, job := range jobs {
		completeness := 1.0
		if job.Err != nil {
			completeness = 0
		}
		batch.Queue(`
			INSERT INTO raw_evidence (id, collection_run_id, olt_id, command, captured_at, parser_version, completeness, body)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO NOTHING`,
			fmt.Sprintf("run%d-%s", runID, job.ResultID), runID, oltID, job.Line, at, version, completeness, job.Raw)
	}

	results := tx.SendBatch(ctx, batch)
	for range jobs {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return fmt.Errorf("insert evidence: %w", err)
		}
	}
	if err := results.Close(); err != nil {
		return fmt.Errorf("close evidence batch: %w", err)
	}
	return tx.Commit(ctx)
}

func (in Ingest) writePon(ctx context.Context, runID int64, oltID string, at time.Time, pon string, work *ponWork) error {
	tx, err := in.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := ensurePon(ctx, tx, oltID, pon); err != nil {
		return err
	}

	// Order within the batch is preserved: identity and status create the ONU
	// rows that the measurements reference.
	batch := &pgx.Batch{}
	queued := 0

	flush := func() error {
		if queued == 0 {
			return nil
		}
		results := tx.SendBatch(ctx, batch)
		for i := 0; i < queued; i++ {
			if _, err := results.Exec(); err != nil {
				_ = results.Close()
				return err
			}
		}
		if err := results.Close(); err != nil {
			return err
		}
		batch = &pgx.Batch{}
		queued = 0
		return nil
	}

	for _, row := range work.status {
		status := "offline"
		if row.Online {
			status = "online"
		}
		batch.Queue(`
			INSERT INTO onus (id, olt_id, pon_id, onu_number, mac, status, status_observed_at, identity_source)
			VALUES ($1, $2, $3, $4, $5::macaddr, $6::onu_status, $7, 'status-only')
			ON CONFLICT (id) DO UPDATE
			   SET status = EXCLUDED.status,
			       status_observed_at = EXCLUDED.status_observed_at,
			       mac = EXCLUDED.mac,
			       identity_source = CASE WHEN onus.identity_source = 'basic-info'
			                              THEN onus.identity_source ELSE 'status-only' END`,
			onuID(oltID, pon, row.OnuID), oltID, ponID(oltID, pon), row.OnuID, row.MAC, status, at)
		queued++
		if queued >= BatchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	for _, row := range work.identity {
		batch.Queue(`
			INSERT INTO onus (id, olt_id, pon_id, onu_number, mac, status,
			                  vendor_id, model, hardware_ver, software_ver,
			                  onu_type, interface_type, identity_source, threshold_model)
			VALUES ($1, $2, $3, $4, '00:00:00:00:00:00'::macaddr, 'unknown',
			        $5, $6, $7, $8, $9, $10, 'basic-info', $11)
			ON CONFLICT (id) DO UPDATE
			   SET vendor_id = EXCLUDED.vendor_id, model = EXCLUDED.model,
			       hardware_ver = EXCLUDED.hardware_ver, software_ver = EXCLUDED.software_ver,
			       onu_type = EXCLUDED.onu_type, interface_type = EXCLUDED.interface_type,
			       identity_source = 'basic-info', threshold_model = EXCLUDED.threshold_model`,
			onuID(oltID, pon, row.OnuID), oltID, ponID(oltID, pon), row.OnuID,
			row.Vendor, row.Model, row.HardwareVer, row.SoftwareVer,
			row.Type, row.InterfaceType, row.Vendor+"/"+row.Model)
		queued++
		if queued >= BatchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	for _, row := range work.optical {
		batch.Queue(`
			INSERT INTO optical_measurements
			    (onu_id, collection_run_id, rx_power_dbm, tx_power_dbm, temperature_c,
			     supply_voltage_v, tx_bias_ma, observed_at, timestamp_quality, evidence_ref)
			SELECT $1, $2, $3, $4, $5, $6, $7, $8, 'untrusted', $9
			 WHERE EXISTS (SELECT 1 FROM onus WHERE id = $1)`,
			onuID(oltID, pon, row.OnuID), runID, row.RxDbm, row.TxDbm,
			row.TemperatureC, row.SupplyVoltageV, row.TxBiasMa, at,
			fmt.Sprintf("run%d-opm-%s", runID, pon))
		queued++
		if queued >= BatchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}

	if err := flush(); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
