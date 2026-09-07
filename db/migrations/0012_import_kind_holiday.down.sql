DELETE FROM import_rejection WHERE import_run_id IN (SELECT import_run_id FROM import_run WHERE kind = 'holiday');
DELETE FROM import_run WHERE kind = 'holiday';
ALTER TABLE import_run DROP CONSTRAINT import_run_kind_known;
ALTER TABLE import_run
    ADD CONSTRAINT import_run_kind_known
    CHECK (kind IN ('transactions', 'target_year', 'target_month'));
