// Package postgres is the pgx implementation of store.Store.
//
// The SQL is deliberately plain: scalar subqueries instead of CTEs, no dynamic
// SQL, no string formatting of identifiers. Every query is a constant so it can
// be reviewed as SQL rather than puzzled out from Go string concatenation.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abuzafarhaqq/olt-monitor/backend/internal/domain"
	"github.com/abuzafarhaqq/olt-monitor/backend/internal/store"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Close() { s.pool.Close() }

// Pool exposes the pool for migrations and seeding. Handlers never see it.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// --- queries ------------------------------------------------------------

const oltsQuery = `
SELECT o.id, o.name, o.vendor, o.model, o.technology,
       host(o.management_address), o.connection,
       COALESCE(st.reachable, false),
       COALESCE(st.firmware, ''),
       COALESCE(st.hardware_version, ''),
       st.uptime_seconds,
       st.device_time,
       st.observed_at,
       COALESCE(st.clock_quality::text, 'missing'),
       (SELECT count(*)::int FROM pon_ports p WHERE p.olt_id = o.id),
       -- A device that was never collected has no successful status; 'failed'
       -- is the honest value, not 'successful' and not NULL.
       COALESCE(st.collection_status::text, 'failed')
FROM olts o
LEFT JOIN olt_state st ON st.olt_id = o.id
WHERE o.enabled
ORDER BY o.name`

const ponsQuery = `
SELECT p.id, p.olt_id, p.name,
       (SELECT count(*)::int FROM onus u WHERE u.pon_id = p.id),
       (SELECT count(*)::int FROM onus u WHERE u.pon_id = p.id AND u.status = 'online'),
       (SELECT count(*)::int FROM onus u WHERE u.pon_id = p.id AND u.status = 'offline'),
       (SELECT count(*)::int FROM onus u WHERE u.pon_id = p.id AND u.status = 'unknown'),
       -- Low optical requires a calibrated threshold for the ONU's vendor and
       -- the OLT's technology. No threshold means no count, never a guess.
       (SELECT count(*)::int
          FROM onus u
          JOIN olt_thresholds t
            ON t.vendor = COALESCE(u.vendor, '') AND t.technology = o.technology
          JOIN LATERAL (
                SELECT m.rx_power_dbm
                  FROM optical_measurements m
                 WHERE m.onu_id = u.id AND m.rx_power_dbm IS NOT NULL
                 ORDER BY m.observed_at DESC
                 LIMIT 1
          ) m ON true
         WHERE u.pon_id = p.id AND m.rx_power_dbm <= t.warning_dbm),
       (SELECT count(DISTINCT e.onu_id)::int
          FROM events e
         WHERE e.pon_id = p.id
           AND e.type = 'onu-dying-gasp'
           AND e.ingested_at > now() - interval '24 hours'),
       (SELECT count(*)::int FROM onus u WHERE u.pon_id = p.id AND u.deregistrations_24h >= 3),
       st.observed_at, st.device_time, COALESCE(st.clock_quality::text, 'missing'),
       COALESCE(st.collection_status::text, 'failed')
FROM pon_ports p
JOIN olts o ON o.id = p.olt_id
LEFT JOIN olt_state st ON st.olt_id = o.id
ORDER BY p.olt_id, p.name`

// onuColumns is shared by the per-PON and single-ONU queries so they cannot
// drift apart in column order.
const onuColumns = `
       u.id, u.olt_id, u.pon_id, u.onu_number, u.mac::text,
       COALESCE(u.vendor, ''), COALESCE(u.model, ''),
       u.status::text,
       m.rx_power_dbm, m.distance_m,
       u.optical_supported, u.last_event,
       u.last_online_at, u.last_registration_at, u.last_deregistration_at,
       u.deregistrations_24h,
       COALESCE(st.collection_status::text, 'failed'),
       COALESCE(u.vendor_id, ''), COALESCE(u.onu_type, ''), COALESCE(u.interface_type, ''),
       COALESCE(u.identity_source, 'unknown'), COALESCE(u.threshold_model, '')`

const onuFrom = `
FROM onus u
JOIN olts o ON o.id = u.olt_id
LEFT JOIN olt_state st ON st.olt_id = u.olt_id
LEFT JOIN LATERAL (
      SELECT ms.rx_power_dbm, ms.distance_m
        FROM optical_measurements ms
       WHERE ms.onu_id = u.id
       ORDER BY ms.observed_at DESC
       LIMIT 1
) m ON true`

const onusByPonQuery = `SELECT ` + onuColumns + onuFrom + `
WHERE u.pon_id = $1
ORDER BY u.onu_number`

const onuQuery = `SELECT ` + onuColumns + onuFrom + `
WHERE u.id = $1`

const eventsQuery = `
SELECT e.id, e.type, e.severity::text, e.olt_id, e.pon_id, u.onu_number,
       e.message, e.device_time, e.ingested_at, e.timestamp_quality::text, e.evidence_ref
FROM events e
LEFT JOIN onus u ON u.id = e.onu_id
ORDER BY e.ingested_at DESC`

// incidentsSelect carries no ORDER BY: the single-incident query appends a
// WHERE clause, and appending it after an ORDER BY would be a syntax error.
const incidentsSelect = `
SELECT i.id, i.title, i.state::text, i.severity::text, i.olt_id, i.pon_id,
       i.opened_at, i.hypothesis
FROM incidents i`

const incidentsQuery = incidentsSelect + ` ORDER BY i.opened_at DESC`

const incidentQuery = incidentsSelect + ` WHERE i.id = $1`

// Latest run per (OLT, job type). DISTINCT ON keeps the newest row per group.
const jobsQuery = `
SELECT DISTINCT ON (r.olt_id, r.job_type)
       r.id::text, r.olt_id, r.job_type, r.result::text, r.started_at,
       r.error_kind, r.error_message, r.state_left_unchanged,
       CASE WHEN r.finished_at IS NULL THEN NULL
            ELSE EXTRACT(EPOCH FROM (r.finished_at - r.started_at)) * 1000
       END AS duration_ms
FROM collection_runs r
ORDER BY r.olt_id, r.job_type, r.started_at DESC`

const opticalHistoryQuery = `
SELECT observed_at, rx_power_dbm
FROM optical_measurements
WHERE onu_id = $1
  AND observed_at > now() - interval '24 hours'
ORDER BY observed_at`

// --- helpers ------------------------------------------------------------

func timestamps(deviceTime *time.Time, ingestedAt *time.Time, quality string) *domain.DeviceTimestamps {
	if ingestedAt == nil {
		return nil
	}
	return &domain.DeviceTimestamps{
		DeviceTime: deviceTime,
		IngestedAt: *ingestedAt,
		Quality:    domain.TimestampQuality(quality),
	}
}

// --- Store implementation -----------------------------------------------

func (s *Store) Olts(ctx context.Context) ([]domain.Olt, error) {
	rows, err := s.pool.Query(ctx, oltsQuery)
	if err != nil {
		return nil, fmt.Errorf("query olts: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Olt, 0, 8)
	for rows.Next() {
		var (
			olt        domain.Olt
			uptime     *int64
			deviceTime *time.Time
			observedAt *time.Time
			quality    string
		)
		if err := rows.Scan(
			&olt.ID, &olt.Name, &olt.Vendor, &olt.Model, &olt.Technology,
			&olt.ManagementAddress, &olt.Connection,
			&olt.Reachable, &olt.Firmware, &olt.HardwareVersion,
			&uptime, &deviceTime, &observedAt, &quality,
			&olt.PonCount, &olt.CollectionStatus,
		); err != nil {
			return nil, fmt.Errorf("scan olt: %w", err)
		}
		olt.UptimeSeconds = uptime
		olt.LastCollection = timestamps(deviceTime, observedAt, quality)
		out = append(out, olt)
	}
	return out, rows.Err()
}

func (s *Store) Olt(ctx context.Context, id string) (domain.Olt, error) {
	olts, err := s.Olts(ctx)
	if err != nil {
		return domain.Olt{}, err
	}
	for _, olt := range olts {
		if olt.ID == id {
			return olt, nil
		}
	}
	return domain.Olt{}, store.ErrNotFound
}

func (s *Store) allPons(ctx context.Context) ([]domain.Pon, error) {
	rows, err := s.pool.Query(ctx, ponsQuery)
	if err != nil {
		return nil, fmt.Errorf("query pons: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Pon, 0, 8)
	for rows.Next() {
		var (
			pon        domain.Pon
			deviceTime *time.Time
			observedAt *time.Time
			quality    string
		)
		if err := rows.Scan(
			&pon.ID, &pon.OltID, &pon.Name,
			&pon.Counts.Total, &pon.Counts.Online, &pon.Counts.Offline, &pon.Counts.Unknown,
			&pon.LowOpticalCount, &pon.DyingGaspCount24h, &pon.FlappingCount,
			&observedAt, &deviceTime, &quality,
			&pon.CollectionStatus,
		); err != nil {
			return nil, fmt.Errorf("scan pon: %w", err)
		}
		pon.LastCollection = timestamps(deviceTime, observedAt, quality)
		out = append(out, pon)
	}
	return out, rows.Err()
}

func (s *Store) PonsByOlt(ctx context.Context, oltID string) ([]domain.Pon, error) {
	pons, err := s.allPons(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Pon, 0, 4)
	for _, pon := range pons {
		if pon.OltID == oltID {
			out = append(out, pon)
		}
	}
	if len(out) == 0 {
		return nil, store.ErrNotFound
	}
	return out, nil
}

func scanOnu(row pgx.Row) (domain.Onu, error) {
	var (
		onu          domain.Onu
		rx           *float64
		distance     *int
		lastEvent    *string
		lastOnline   *time.Time
		lastReg      *time.Time
		lastDereg    *time.Time
		collectionSt string
	)
	if err := row.Scan(
		&onu.ID, &onu.OltID, &onu.PonID, &onu.OnuID, &onu.MAC,
		&onu.Vendor, &onu.Model, &onu.Status,
		&rx, &distance, &onu.OpticalSupported, &lastEvent,
		&lastOnline, &lastReg, &lastDereg,
		&onu.Deregistrations24h, &collectionSt,
		&onu.VendorID, &onu.OnuType, &onu.InterfaceType, &onu.IdentitySource, &onu.ThresholdModel,
	); err != nil {
		return domain.Onu{}, err
	}
	onu.RxPowerDbm = rx
	onu.DistanceMeters = distance
	onu.LastEvent = lastEvent
	onu.LastOnlineAt = timestamps(nil, lastOnline, "trusted")
	onu.LastRegistrationAt = timestamps(nil, lastReg, "trusted")
	onu.LastDeregistrationAt = timestamps(nil, lastDereg, "trusted")
	onu.CollectionStatus = domain.CollectionStatus(collectionSt)
	onu.Provisioned = onu.IdentitySource == "basic-info"
	return onu, nil
}

func (s *Store) OnusByPon(ctx context.Context, ponID string) ([]domain.Onu, error) {
	rows, err := s.pool.Query(ctx, onusByPonQuery, ponID)
	if err != nil {
		return nil, fmt.Errorf("query onus: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Onu, 0, 16)
	for rows.Next() {
		onu, err := scanOnu(rows)
		if err != nil {
			return nil, fmt.Errorf("scan onu: %w", err)
		}
		out = append(out, onu)
	}
	return out, rows.Err()
}

func (s *Store) allOnus(ctx context.Context) ([]domain.Onu, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+onuColumns+onuFrom+` ORDER BY u.olt_id, u.pon_id, u.onu_number`)
	if err != nil {
		return nil, fmt.Errorf("query onus: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Onu, 0, 64)
	for rows.Next() {
		onu, err := scanOnu(rows)
		if err != nil {
			return nil, fmt.Errorf("scan onu: %w", err)
		}
		out = append(out, onu)
	}
	return out, rows.Err()
}

func (s *Store) Onu(ctx context.Context, id string) (domain.Onu, error) {
	onu, err := scanOnu(s.pool.QueryRow(ctx, onuQuery, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Onu{}, store.ErrNotFound
	}
	if err != nil {
		return domain.Onu{}, fmt.Errorf("query onu: %w", err)
	}
	return onu, nil
}

func (s *Store) OpticalHistory(ctx context.Context, onuID string) ([]domain.OpticalPoint, error) {
	rows, err := s.pool.Query(ctx, opticalHistoryQuery, onuID)
	if err != nil {
		return nil, fmt.Errorf("query optical history: %w", err)
	}
	defer rows.Close()

	out := make([]domain.OpticalPoint, 0, 24)
	for rows.Next() {
		var point domain.OpticalPoint
		if err := rows.Scan(&point.At, &point.RxPowerDbm); err != nil {
			return nil, fmt.Errorf("scan optical point: %w", err)
		}
		out = append(out, point)
	}
	return out, rows.Err()
}

func (s *Store) Events(ctx context.Context) ([]domain.Event, error) {
	rows, err := s.pool.Query(ctx, eventsQuery)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Event, 0, 32)
	for rows.Next() {
		var (
			event      domain.Event
			ponID      *string
			onuNumber  *int
			deviceTime *time.Time
			quality    string
		)
		if err := rows.Scan(
			&event.ID, &event.Type, &event.Severity, &event.OltID, &ponID, &onuNumber,
			&event.Message, &deviceTime, &event.Timestamps.IngestedAt, &quality, &event.EvidenceRef,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		event.PonID = ponID
		event.OnuID = onuNumber
		event.Timestamps.DeviceTime = deviceTime
		event.Timestamps.Quality = domain.TimestampQuality(quality)
		out = append(out, event)
	}
	return out, rows.Err()
}

func (s *Store) Incidents(ctx context.Context) ([]domain.Incident, error) {
	return s.queryIncidents(ctx, incidentsQuery)
}

func (s *Store) Incident(ctx context.Context, id string) (domain.Incident, error) {
	incidents, err := s.queryIncidents(ctx, incidentQuery, id)
	if err != nil {
		return domain.Incident{}, err
	}
	if len(incidents) == 0 {
		return domain.Incident{}, store.ErrNotFound
	}
	return incidents[0], nil
}

func (s *Store) queryIncidents(ctx context.Context, sql string, args ...any) ([]domain.Incident, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query incidents: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Incident, 0, 8)
	for rows.Next() {
		var (
			incident domain.Incident
			ponID    *string
			hypoth   *string
		)
		if err := rows.Scan(
			&incident.ID, &incident.Title, &incident.State, &incident.Severity,
			&incident.OltID, &ponID, &incident.OpenedAt, &hypoth,
		); err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		incident.PonID = ponID
		incident.Hypothesis = hypoth
		incident.Notes = []domain.IncidentNote{}
		incident.EvidenceEventIDs = []string{}
		out = append(out, incident)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Notes and evidence are fetched in one pass each rather than per incident:
	// a page with 200 incidents must not make 400 queries.
	for i := range out {
		if err := s.attachIncidentDetail(ctx, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) attachIncidentDetail(ctx context.Context, incident *domain.Incident) error {
	noteRows, err := s.pool.Query(ctx,
		`SELECT id, author, created_at, body FROM incident_notes WHERE incident_id = $1 ORDER BY created_at`,
		incident.ID)
	if err != nil {
		return fmt.Errorf("query incident notes: %w", err)
	}
	defer noteRows.Close()

	for noteRows.Next() {
		var note domain.IncidentNote
		if err := noteRows.Scan(&note.ID, &note.Author, &note.At, &note.Text); err != nil {
			return fmt.Errorf("scan incident note: %w", err)
		}
		incident.Notes = append(incident.Notes, note)
	}
	if err := noteRows.Err(); err != nil {
		return err
	}

	evidenceRows, err := s.pool.Query(ctx,
		`SELECT event_id FROM incident_evidence WHERE incident_id = $1 ORDER BY event_id`,
		incident.ID)
	if err != nil {
		return fmt.Errorf("query incident evidence: %w", err)
	}
	defer evidenceRows.Close()

	for evidenceRows.Next() {
		var eventID string
		if err := evidenceRows.Scan(&eventID); err != nil {
			return fmt.Errorf("scan incident evidence: %w", err)
		}
		incident.EvidenceEventIDs = append(incident.EvidenceEventIDs, eventID)
	}
	return evidenceRows.Err()
}

func (s *Store) CollectionJobs(ctx context.Context) ([]domain.CollectionJob, error) {
	rows, err := s.pool.Query(ctx, jobsQuery)
	if err != nil {
		return nil, fmt.Errorf("query jobs: %w", err)
	}
	defer rows.Close()

	out := make([]domain.CollectionJob, 0, 16)
	for rows.Next() {
		var (
			job        domain.CollectionJob
			kind       *string
			message    *string
			durationMs *float64
		)
		if err := rows.Scan(
			&job.ID, &job.OltID, &job.Type, &job.Result, &job.LastRunAt,
			&kind, &message, &job.StateLeftUnchanged, &durationMs,
		); err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		if durationMs != nil {
			ms := int64(*durationMs)
			job.DurationMs = &ms
		}
		if kind != nil || message != nil {
			jobErr := &domain.JobError{}
			if kind != nil {
				jobErr.Kind = *kind
			}
			if message != nil {
				jobErr.Message = *message
			}
			job.Error = jobErr
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

// Overview assembles the dashboard payload from the individual queries. It is
// several round trips today; when the page gets slow, this is the method that
// becomes one query or one cached snapshot.
func (s *Store) Overview(ctx context.Context) (domain.Overview, error) {
	olts, err := s.Olts(ctx)
	if err != nil {
		return domain.Overview{}, err
	}
	pons, err := s.allPons(ctx)
	if err != nil {
		return domain.Overview{}, err
	}
	onus, err := s.allOnus(ctx)
	if err != nil {
		return domain.Overview{}, err
	}
	events, err := s.Events(ctx)
	if err != nil {
		return domain.Overview{}, err
	}
	incidents, err := s.Incidents(ctx)
	if err != nil {
		return domain.Overview{}, err
	}

	return domain.Overview{
		GeneratedAt:       time.Now().UTC(),
		Olts:              olts,
		Pons:              pons,
		Onus:              onus,
		Events:            events,
		Incidents:         incidents,
		AvailabilityTrend: trend(onus, time.Now().UTC()),
	}, nil
}

// trend builds hourly buckets from the current population. It is a placeholder
// until availability history is stored — and it says so by taking `now`.
func trend(onus []domain.Onu, now time.Time) []domain.AvailabilityPoint {
	online, offline, unknown := 0, 0, 0
	for _, onu := range onus {
		switch onu.Status {
		case domain.OnuOnline:
			online++
		case domain.OnuOffline:
			offline++
		default:
			unknown++
		}
	}

	points := make([]domain.AvailabilityPoint, 0, 24)
	for hour := 23; hour >= 0; hour-- {
		at := now.Add(-time.Duration(hour) * time.Hour)
		points = append(points, domain.AvailabilityPoint{
			HourLabel: at.Format("15:04"),
			Online:    online,
			Offline:   offline,
			Unknown:   unknown,
		})
	}
	return points
}
