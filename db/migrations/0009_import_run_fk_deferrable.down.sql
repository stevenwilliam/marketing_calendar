ALTER TABLE history_txn DROP CONSTRAINT history_txn_import_run_id_fkey;
ALTER TABLE history_txn
    ADD CONSTRAINT history_txn_import_run_id_fkey
    FOREIGN KEY (import_run_id) REFERENCES import_run (import_run_id);
