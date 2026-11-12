-- A failed attempt is not an observation.
--
-- observed_at was being overwritten on every attempt, so a device that had been
-- unreachable for days still showed "just now" next to a clock from its last
-- successful collection. Two timestamps, two meanings.
ALTER TABLE olt_state ADD COLUMN last_success_at TIMESTAMPTZ;

UPDATE olt_state SET last_success_at = observed_at WHERE collection_status IN ('successful', 'partial');
