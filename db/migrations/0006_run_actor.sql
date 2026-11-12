-- Who asked for this run.
--
-- The audit log records "olt.collect" with an actor, but the Logs page reads
-- collection_runs, so the two were disconnected: you could see that a run
-- happened and not who triggered it.
ALTER TABLE collection_runs ADD COLUMN triggered_by TEXT;

CREATE INDEX collection_runs_actor_idx ON collection_runs (triggered_by, started_at DESC);
