package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store"
)

// RunSummary is one row of the collection log.
type RunSummary struct {
	ID            int64      `json:"id"`
	OltID         string     `json:"oltId"`
	OltName       string     `json:"oltName"`
	JobType       string     `json:"jobType"`
	Result        string     `json:"result"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
	DurationMs    *int64     `json:"durationMs"`
	ErrorMessage  *string    `json:"errorMessage"`
	TriggeredBy   *string    `json:"triggeredBy"`
	EvidenceCount int        `json:"evidenceCount"`
}

// EvidenceRow is one captured command output.
type EvidenceRow struct {
	ID            string    `json:"id"`
	Command       string    `json:"command"`
	CapturedAt    time.Time `json:"capturedAt"`
	ParserVersion string    `json:"parserVersion"`
	Completeness  float64   `json:"completeness"`
	Body          string    `json:"body"`
}

func (s *Store) ListRuns(ctx context.Context, limit int) ([]RunSummary, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.olt_id, COALESCE(o.name, r.olt_id), r.job_type, r.result::text,
		       r.started_at, r.finished_at, r.error_message,
		       CASE WHEN r.finished_at IS NULL THEN NULL
		            ELSE (EXTRACT(EPOCH FROM (r.finished_at - r.started_at)) * 1000)::bigint END,
		       (SELECT count(*)::int FROM raw_evidence e WHERE e.collection_run_id = r.id),
		       r.triggered_by
		  FROM collection_runs r
		  LEFT JOIN olts o ON o.id = r.olt_id
		 ORDER BY r.started_at DESC
		 LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	out := make([]RunSummary, 0, limit)
	for rows.Next() {
		var run RunSummary
		if err := rows.Scan(&run.ID, &run.OltID, &run.OltName, &run.JobType, &run.Result,
			&run.StartedAt, &run.FinishedAt, &run.ErrorMessage, &run.DurationMs, &run.EvidenceCount, &run.TriggeredBy); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// RunEvidence returns every captured command for one run, in id order — which is
// execution order, because the ingest inserts them sequentially.
func (s *Store) RunEvidence(ctx context.Context, runID int64) ([]EvidenceRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, command, captured_at, parser_version, completeness, body
		  FROM raw_evidence
		 WHERE collection_run_id = $1
		 ORDER BY id`, runID)
	if err != nil {
		return nil, fmt.Errorf("run evidence: %w", err)
	}
	defer rows.Close()

	out := make([]EvidenceRow, 0, 16)
	for rows.Next() {
		var row EvidenceRow
		if err := rows.Scan(&row.ID, &row.Command, &row.CapturedAt, &row.ParserVersion,
			&row.Completeness, &row.Body); err != nil {
			return nil, fmt.Errorf("scan evidence: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) RunExists(ctx context.Context, runID int64) error {
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM collection_runs WHERE id = $1)`, runID).
		Scan(&exists); err != nil {
		return fmt.Errorf("check run: %w", err)
	}
	if !exists {
		return store.ErrNotFound
	}
	return nil
}

var _ = errors.Is
var _ = pgx.ErrNoRows
