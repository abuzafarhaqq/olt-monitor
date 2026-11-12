-- Host keys, managed from the UI.
--
-- The CLI had -accept-key; a UI user cannot type that. Same policy, moved to the
-- device record: trust the key on first connect, store its fingerprint, and show
-- it so a human can compare it. Verification never becomes "off" — a key that
-- changes later is a mismatch and refuses to connect.
ALTER TABLE olts
    ADD COLUMN trust_host_key      BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN host_key_fingerprint TEXT,
    ADD COLUMN host_key_seen_at     TIMESTAMPTZ;

-- Discovered commands belong to a run, so the crawl is auditable like a collection.
ALTER TABLE collection_runs DROP CONSTRAINT IF EXISTS collection_runs_result_check;
