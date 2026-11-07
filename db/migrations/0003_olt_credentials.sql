-- Device credentials, encrypted at rest, in their own table.
--
-- Layout decisions that matter more than the column names:
--   * separate table, because `SELECT * FROM olts` runs in psql, in logs and in
--     debuggers far more often than anyone expects;
--   * ciphertext only — the key lives in the environment, never in a row, so a
--     database dump on its own yields nothing usable;
--   * key_version, so a key can be rotated without rewriting every row at once.

CREATE TABLE olt_credentials (
    olt_id      TEXT PRIMARY KEY REFERENCES olts(id) ON DELETE CASCADE,
    username    TEXT NOT NULL,
    secret_enc  BYTEA NOT NULL,
    key_version INT   NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by  UUID REFERENCES users(id) ON DELETE SET NULL
);

-- `olts.credential_ref` was a pointer to a store that never existed. The row
-- above is the store, so the pointer is now misleading.
ALTER TABLE olts DROP COLUMN credential_ref;
