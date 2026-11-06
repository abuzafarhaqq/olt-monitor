package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Seed replaces the contents of the database with the demo fixture.
//
// It mirrors internal/store/memory so the frontend sees the same numbers
// whichever store is running. Both fixtures are deliberately awkward: one
// unreachable OLT, one partial run, an ONU with no optical command, a device
// clock from 2000 and two identical events.
//
// Never run this against a database with real data in it.
func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin seed: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const statements = `
TRUNCATE audit_log, incident_notes, incident_evidence, incidents,
         events, optical_measurements, raw_evidence, collection_runs,
         olt_thresholds, onus, pon_ports, olt_state, olts
  RESTART IDENTITY CASCADE;

INSERT INTO olts (id, name, vendor, model, technology, management_address, management_port,
                  connection, collection_profile)
VALUES
 ('olt-1', 'EPON-OLT-01', 'VSOL',  'EPON-OLT', 'EPON', '192.0.2.11', 22, 'ssh',    'standard-5m'),
 ('olt-2', 'GPON-OLT-02', 'C-Data','FD-OLT',   'GPON', '192.0.2.14', 23, 'telnet', 'standard-5m');

INSERT INTO olt_state (olt_id, reachable, firmware, hardware_version, uptime_seconds,
                       device_time, clock_quality, observed_at, collection_status)
VALUES
 ('olt-1', TRUE,  'demo-2.03.75', 'V1.1.4', 3542400,
  '2000-05-20 04:12:33+00', 'untrusted', now() - interval '3 minutes',  'successful'),
 ('olt-2', FALSE, 'unknown',      'unknown', NULL,
  NULL,                     'missing',   now() - interval '21 minutes', 'failed');

INSERT INTO olt_thresholds (vendor, technology, warning_dbm, critical_dbm) VALUES
 ('VSOL',   'EPON', -24, -27),
 ('VSOL',   'GPON', -27, -29),
 ('C-Data', 'GPON', -26, -28);

INSERT INTO pon_ports (id, olt_id, name) VALUES
 ('olt-1-pon1', 'olt-1', 'epon 0/1'),
 ('olt-1-pon2', 'olt-1', 'epon 0/2'),
 ('olt-2-pon1', 'olt-2', 'gpon 0/1');

INSERT INTO onus (id, olt_id, pon_id, onu_number, mac, vendor, model, status,
                  status_observed_at, optical_supported, deregistrations_24h, last_event,
                  last_online_at)
VALUES
 ('olt-1-pon1-onu1', 'olt-1', 'olt-1-pon1', 1, '02:1A:2B:00:00:01', 'VSOL',   'HG323',   'online',
  now() - interval '3 minutes', TRUE, 0, 'onu-register',   now() - interval '3 minutes'),
 ('olt-1-pon1-onu2', 'olt-1', 'olt-1-pon1', 2, '02:1A:2B:00:00:02', 'VSOL',   'HG323',   'online',
  now() - interval '3 minutes', TRUE, 0, 'onu-register',   now() - interval '3 minutes'),
 ('olt-1-pon2-onu3', 'olt-1', 'olt-1-pon2', 3, '02:1A:2B:00:00:03', 'VSOL',   'HG323',   'online',
  now() - interval '3 minutes', FALSE, 0, 'onu-register',  now() - interval '3 minutes'),
 ('olt-1-pon2-onu4', 'olt-1', 'olt-1-pon2', 4, '02:1A:2B:00:00:04', 'VSOL',   'HG323',   'offline',
  now() - interval '4 minutes', TRUE, 3, 'onu-deregister', now() - interval '40 minutes'),
 ('olt-2-pon1-onu1', 'olt-2', 'olt-2-pon1', 1, '02:1A:2B:99:00:01', 'C-Data', 'FD611GW', 'unknown',
  NULL, TRUE,  0, NULL, NULL),
 ('olt-2-pon1-onu2', 'olt-2', 'olt-2-pon1', 2, '02:1A:2B:99:00:02', 'C-Data', 'FD611GW', 'unknown',
  NULL, TRUE,  0, NULL, NULL);

-- Latest optical sample per ONU. ONU 3 has none: the adapter has no optical
-- command for that model, which is not the same as a reading of zero.
INSERT INTO optical_measurements (onu_id, rx_power_dbm, distance_m, observed_at,
                                  timestamp_quality, device_time)
VALUES
 ('olt-1-pon1-onu1', -18.4,  420, now() - interval '3 minutes', 'untrusted', '2000-05-20 04:12:33+00'),
 ('olt-1-pon1-onu2', -31.4, 1180, now() - interval '3 minutes', 'untrusted', '2000-05-20 04:12:33+00'),
 ('olt-1-pon2-onu4', NULL,   NULL, now() - interval '4 minutes', 'untrusted', '2000-05-20 04:12:33+00');

INSERT INTO collection_runs (olt_id, job_type, started_at, finished_at, result,
                             error_kind, error_message, state_left_unchanged)
VALUES
 ('olt-1', 'onu-status',  now() - interval '3 minutes',  now() - interval '3 minutes',  'successful', NULL,      NULL, NULL),
 ('olt-1', 'onu-optical', now() - interval '3 minutes',  now() - interval '3 minutes',  'partial',    'parser',  '9 of 10 rows parsed', 1),
 ('olt-2', 'onu-status',  now() - interval '21 minutes', now() - interval '20 minutes', 'failed',     'timeout', 'no prompt within 15 s', 2);

INSERT INTO raw_evidence (id, olt_id, command, captured_at, parser_version, completeness, body)
VALUES
 ('ev-1', 'olt-1', 'show onu status all', now() - interval '3 minutes',
  'vsol-epon@0.6.2', 1.0, E'PON  ONU  Status\n0/1  1    online\n');

-- Two captures of one event, two different evidence references.
INSERT INTO events (id, olt_id, pon_id, onu_id, type, severity, message, device_time,
                    ingested_at, timestamp_quality, evidence_ref)
VALUES
 ('evt-1', 'olt-1', 'olt-1-pon2', 'olt-1-pon2-onu4', 'onu-deregister', 'critical',
  'ONU deregistered — dying gasp recorded', '2000-05-20 04:12:33+00',
  now() - interval '4 minutes', 'untrusted', 'evidence/olt-1/syslog-deregister/2026-11-06T01-12Z.txt'),
 ('evt-2', 'olt-1', 'olt-1-pon2', 'olt-1-pon2-onu4', 'onu-deregister', 'critical',
  'ONU deregistered — dying gasp recorded', '2000-05-20 04:12:33+00',
  now() - interval '4 minutes', 'untrusted', 'evidence/olt-1/syslog-dying-gasp/2026-11-06T01-12Z.txt'),
 ('evt-3', 'olt-2', NULL, NULL, 'olt-unreachable', 'critical',
  'OLT unreachable — Telnet timeout after 3 attempts', NULL,
  now() - interval '21 minutes', 'missing', 'evidence/olt-2/collector/2026-11-06T00-55Z.log');

INSERT INTO incidents (id, title, state, severity, olt_id, pon_id, opened_at, hypothesis)
VALUES
 ('INC-2411-007', 'PON 0/2 flapping with dying-gasp cluster', 'investigating', 'critical',
  'olt-1', 'olt-1-pon2', now() - interval '3 hours',
  'Shared PON outage suspected, not confirmed: the OLT is reachable and PON 0/1 is unaffected.');

INSERT INTO incident_evidence (incident_id, event_id) VALUES
 ('INC-2411-007', 'evt-1'),
 ('INC-2411-007', 'evt-2');

INSERT INTO incident_notes (id, incident_id, author, created_at, body) VALUES
 ('note-1', 'INC-2411-007', 'NOC Operator', now() - interval '90 minutes',
  'Dying gasp on ONU 4 only; the others simply deregistered.');
`

	if _, err := tx.Exec(ctx, statements); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit seed: %w", err)
	}
	return nil
}
