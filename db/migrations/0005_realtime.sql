-- A run now exists before it finishes: the API returns 202 with a run id while
-- the collection is still dialling, so the row has to be insertable in a
-- 'running' state and updated at the end.
ALTER TYPE job_result ADD VALUE IF NOT EXISTS 'running';

-- Contract for the live terminal: one row per streamed run, so the hub knows
-- which run belongs to which OLT and a second click can be refused.
ALTER TABLE collection_runs
    ADD COLUMN live BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN streamed_events INT NOT NULL DEFAULT 0;
