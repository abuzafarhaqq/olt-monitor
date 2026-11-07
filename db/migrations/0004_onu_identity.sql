-- ONU identity, provisioning state and per-model optical thresholds.
--
-- Two facts from the live EPON drive this migration:
--   1. A single PON carries ONUs from many vendors (26 vendor/model pairs on one
--      OLT), so a threshold keyed on the OLT's vendor is wrong for most of them.
--   2. `show onu status` lists entries that never registered: 53 rows on a port
--      with 31 provisioned ONUs. The device calls them "offline"; they are
--      unprovisioned, and a dashboard that counts them reports a permanent
--      outage on a healthy PON.

ALTER TABLE onus
    ADD COLUMN vendor_id      TEXT,
    ADD COLUMN onu_type       TEXT,   -- SFU / HGU
    ADD COLUMN interface_type TEXT,   -- 1GE, 2FE+1POTS, 1GE+CATV …
    ADD COLUMN hardware_ver   TEXT,
    ADD COLUMN software_ver   TEXT,
    -- Provenance matters more than a boolean: an ONU that never appeared in
    -- basic-info is not the same thing as one we failed to read.
    ADD COLUMN identity_source TEXT NOT NULL DEFAULT 'unknown'
        CHECK (identity_source IN ('unknown', 'basic-info', 'status-only'));

-- Vendor/model is what the optical thresholds are keyed on now.
ALTER TABLE onus ADD COLUMN threshold_model TEXT;

-- Thresholds per ONU model. The OLT-side thresholds stay for the OLT's own
-- optics; this table answers "is this ONU's RX healthy for *this* ONU".
CREATE TABLE onu_thresholds (
    vendor_id    TEXT NOT NULL,
    model        TEXT NOT NULL,
    warning_dbm  REAL NOT NULL,
    critical_dbm REAL NOT NULL,
    source       TEXT NOT NULL DEFAULT 'default',  -- 'default' until measured
    PRIMARY KEY (vendor_id, model),
    CHECK (critical_dbm <= warning_dbm)
);

-- Device capabilities, keyed by what actually determines the command set:
-- vendor + model + software version. A firmware upgrade is a new row, not an
-- assumption that yesterday's commands still work.
CREATE TABLE olt_capabilities (
    id             BIGSERIAL PRIMARY KEY,
    vendor         TEXT NOT NULL,
    model          TEXT NOT NULL,
    -- NULL means "any version", used only when the vendor guarantees stability.
    software_match TEXT,
    technology     TEXT NOT NULL,
    -- JSON array of {"id","line","readOnly"} as discovered from the device.
    commands       JSONB NOT NULL,
    source         TEXT NOT NULL,       -- 'discovery' | 'manual'
    discovered_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    notes          TEXT
);

CREATE INDEX olt_capabilities_lookup_idx ON olt_capabilities (vendor, model);

-- What the one live VSOL EPON reported when asked with '?'. Kept as data, so a
-- second OLT with different firmware can answer differently without a rebuild.
INSERT INTO olt_capabilities (vendor, model, software_match, technology, commands, source, notes)
VALUES
 ('VSOL', 'EPON-OLT', 'V2.03.75R%', 'EPON',
  '[{"id":"version","line":"show version","readOnly":true},
    {"id":"time","line":"show time","readOnly":true},
    {"id":"onu-status","line":"show onu status all","readOnly":true},
    {"id":"onu-status-pon","line":"show onu status pon N","readOnly":true},
    {"id":"opm-diag","line":"show onu opm-diag all","readOnly":true},
    {"id":"opm-diag-pon","line":"show onu opm-diag pon N","readOnly":true},
    {"id":"onu-basic-info","line":"show onu basic-info all","readOnly":true}]',
  'discovery',
  'show onu ? also offers command, pool, timer, register, discover — all act on ONUs and are deliberately excluded');
