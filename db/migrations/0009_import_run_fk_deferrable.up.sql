-- 0009 — make history_txn -> import_run deferrable.
--
-- The importer must write its transactions and its run row in ONE
-- transaction: a crash between them either loses a day of trading or records
-- a run that loaded nothing. But the row counts (inserted, skipped) are only
-- known AFTER the inserts, and import_run is append-only (BR-6.4), so they
-- cannot be back-filled with an UPDATE.
--
-- Deferring the foreign key resolves it: the transactions are inserted first,
-- the run row last with its true counts, and the constraint is checked at
-- COMMIT — by which time both exist. Found by importer integration test,
-- which failed with a 23503 against the live schema.

ALTER TABLE history_txn
    DROP CONSTRAINT history_txn_import_run_id_fkey;

ALTER TABLE history_txn
    ADD CONSTRAINT history_txn_import_run_id_fkey
    FOREIGN KEY (import_run_id) REFERENCES import_run (import_run_id)
    DEFERRABLE INITIALLY DEFERRED;
