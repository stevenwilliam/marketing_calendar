DROP INDEX IF EXISTS import_run_kind_ix;
ALTER TABLE import_run DROP CONSTRAINT IF EXISTS import_run_kind_known;
ALTER TABLE import_run DROP COLUMN IF EXISTS kind;
