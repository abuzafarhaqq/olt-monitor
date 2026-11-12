-- What each device's CLI actually offers, discovered rather than assumed.
--
-- One row per command path per device. A firmware upgrade changes rows here, not
-- Go code: the crawler walks `?` and records what exists, and collection reads
-- this table instead of a compiled-in slice.
CREATE TABLE olt_commands (
    olt_id       TEXT NOT NULL REFERENCES olts(id) ON DELETE CASCADE,
    -- Full command as the device sees it, e.g. "show onu status pon".
    path         TEXT NOT NULL,
    -- What the device's help said about it.
    help         TEXT NOT NULL DEFAULT '',
    -- read | action | param | unknown — action means "never execute this".
    kind         TEXT NOT NULL CHECK (kind IN ('read', 'action', 'param', 'unknown')),
    depth        INT  NOT NULL,
    adapter      TEXT NOT NULL,
    discovered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (olt_id, path)
);

CREATE INDEX olt_commands_kind_idx ON olt_commands (olt_id, kind);
