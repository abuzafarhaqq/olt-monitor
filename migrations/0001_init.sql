-- Initial schema.
--
-- Two ideas drive the design:
--   1. Device state and collection state are separate columns/tables. A failed
--      collection run never writes to onus.status.
--   2. Every stored measurement keeps the device timestamp, the ingestion
--      timestamp, the run id and the raw evidence reference.

BEGIN;

CREATE TYPE onu_status AS ENUM ('online', 'offline', 'unknown');
CREATE TYPE collection_status AS ENUM ('successful', 'partial', 'failed', 'stale', 'unsupported');
CREATE TYPE job_result AS ENUM ('successful', 'partial', 'failed', 'skipped');
CREATE TYPE timestamp_quality AS ENUM ('trusted', 'untrusted', 'missing');
CREATE TYPE event_severity AS ENUM ('info', 'warning', 'critical');
CREATE TYPE incident_state AS ENUM ('new', 'acknowledged', 'investigating', 'resolved', 'closed');

CREATE TABLE olts (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL UNIQUE,
    vendor              TEXT NOT NULL,
    model               TEXT NOT NULL,
    technology          TEXT NOT NULL CHECK (technology IN ('EPON', 'GPON')),
    management_address  INET NOT NULL,
    management_port     INT  NOT NULL DEFAULT 22,
    connection          TEXT NOT NULL CHECK (connection IN ('ssh', 'telnet')),
    -- A reference to a secret, never the secret itself.
    credential_ref      TEXT NOT NULL,
    collection_profile  TEXT NOT NULL DEFAULT 'standard-5m',
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- What the device last told us about itself. One row per OLT, replaced on each
-- successful device-info collection.
CREATE TABLE olt_state (
    olt_id              TEXT PRIMARY KEY REFERENCES olts(id) ON DELETE CASCADE,
    reachable           BOOLEAN NOT NULL,
    firmware            TEXT,
    hardware_version    TEXT,
    uptime_seconds      BIGINT,
    device_time         TIMESTAMPTZ,
    clock_quality       timestamp_quality NOT NULL DEFAULT 'missing',
    observed_at         TIMESTAMPTZ NOT NULL,
    collection_status   collection_status NOT NULL
);

CREATE TABLE pon_ports (
    id                  TEXT PRIMARY KEY,
    olt_id              TEXT NOT NULL REFERENCES olts(id) ON DELETE CASCADE,
    -- Name exactly as the adapter reports it; EPON and GPON name ports
    -- differently and the platform must not normalise that away.
    name                TEXT NOT NULL,
    UNIQUE (olt_id, name)
);

CREATE TABLE onus (
    id                  TEXT PRIMARY KEY,
    olt_id              TEXT NOT NULL REFERENCES olts(id) ON DELETE CASCADE,
    pon_id              TEXT NOT NULL REFERENCES pon_ports(id) ON DELETE CASCADE,
    onu_number          INT  NOT NULL,
    mac                 MACADDR NOT NULL,
    serial              TEXT,
    vendor              TEXT,
    model               TEXT,
    -- Authoritative device state only. Never written by a failed run.
    status              onu_status NOT NULL DEFAULT 'unknown',
    status_observed_at  TIMESTAMPTZ,
    deregistrations_24h INT NOT NULL DEFAULT 0,
    last_online_at      TIMESTAMPTZ,
    last_registration_at TIMESTAMPTZ,
    last_deregistration_at TIMESTAMPTZ
);

CREATE INDEX onus_pon_idx  ON onus (pon_id);
CREATE INDEX onus_olt_idx  ON onus (olt_id);

-- One execution of one job against one device.
CREATE TABLE collection_runs (
    id                  BIGSERIAL PRIMARY KEY,
    olt_id              TEXT NOT NULL REFERENCES olts(id) ON DELETE CASCADE,
    job_type            TEXT NOT NULL,
    started_at          TIMESTAMPTZ NOT NULL,
    finished_at         TIMESTAMPTZ,
    result              job_result NOT NULL,
    error_kind          TEXT,
    error_message       TEXT,
    -- How many ONU rows this run deliberately did not touch.
    state_left_unchanged INT
);

CREATE INDEX collection_runs_olt_idx ON collection_runs (olt_id, started_at DESC);

-- Raw CLI output, kept verbatim. Evidence is never overwritten or truncated.
CREATE TABLE raw_evidence (
    id                  TEXT PRIMARY KEY,
    collection_run_id   BIGINT REFERENCES collection_runs(id) ON DELETE SET NULL,
    olt_id              TEXT NOT NULL REFERENCES olts(id) ON DELETE CASCADE,
    command             TEXT NOT NULL,
    captured_at         TIMESTAMPTZ NOT NULL,
    parser_version      TEXT NOT NULL,
    completeness        REAL NOT NULL CHECK (completeness >= 0 AND completeness <= 1),
    body                TEXT NOT NULL
);

CREATE TABLE optical_measurements (
    id                  BIGSERIAL PRIMARY KEY,
    onu_id              TEXT NOT NULL REFERENCES onus(id) ON DELETE CASCADE,
    collection_run_id   BIGINT REFERENCES collection_runs(id) ON DELETE SET NULL,
    rx_power_dbm        REAL,
    tx_power_dbm        REAL,
    temperature_c       REAL,
    supply_voltage_v    REAL,
    tx_bias_ma          REAL,
    distance_m          INT,
    device_time         TIMESTAMPTZ,
    observed_at         TIMESTAMPTZ NOT NULL,
    timestamp_quality   timestamp_quality NOT NULL,
    evidence_id         TEXT REFERENCES raw_evidence(id) ON DELETE SET NULL
);

CREATE INDEX optical_measurements_onu_time_idx ON optical_measurements (onu_id, observed_at DESC);

CREATE TABLE events (
    id                  TEXT PRIMARY KEY,
    olt_id              TEXT NOT NULL REFERENCES olts(id) ON DELETE CASCADE,
    pon_id              TEXT REFERENCES pon_ports(id) ON DELETE SET NULL,
    onu_id              TEXT REFERENCES onus(id) ON DELETE SET NULL,
    type                TEXT NOT NULL,
    severity            event_severity NOT NULL,
    message             TEXT NOT NULL,
    device_time         TIMESTAMPTZ,
    ingested_at         TIMESTAMPTZ NOT NULL,
    timestamp_quality   timestamp_quality NOT NULL,
    evidence_id         TEXT REFERENCES raw_evidence(id) ON DELETE SET NULL,
    -- Two captures of the same real-world event are two rows, not one.
    UNIQUE (olt_id, type, ingested_at, evidence_id)
);

CREATE INDEX events_time_idx ON events (ingested_at DESC);
CREATE INDEX events_onu_idx  ON events (onu_id, ingested_at DESC);

CREATE TABLE incidents (
    id                  TEXT PRIMARY KEY,
    title               TEXT NOT NULL,
    state               incident_state NOT NULL DEFAULT 'new',
    severity            event_severity NOT NULL,
    olt_id              TEXT NOT NULL REFERENCES olts(id) ON DELETE CASCADE,
    pon_id              TEXT REFERENCES pon_ports(id) ON DELETE SET NULL,
    opened_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- A suspected cause. Text, because a hypothesis is not a state.
    hypothesis          TEXT
);

CREATE TABLE incident_evidence (
    incident_id         TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    event_id            TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    PRIMARY KEY (incident_id, event_id)
);

CREATE TABLE incident_notes (
    id                  TEXT PRIMARY KEY,
    incident_id         TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    author              TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    body                TEXT NOT NULL
);

CREATE TABLE audit_log (
    id                  BIGSERIAL PRIMARY KEY,
    actor               TEXT NOT NULL,
    action              TEXT NOT NULL,
    target              TEXT NOT NULL,
    detail              JSONB,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMIT;
