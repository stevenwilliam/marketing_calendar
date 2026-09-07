-- 0012 — the importer also carries holidays (D49).
--
-- 0010 added the `kind` column with a CHECK naming three kinds. Adding a
-- fourth in Go without widening the CHECK meant the parse succeeded and the
-- INSERT failed with a 23514 — the constraint did its job, which is why the
-- gap surfaced as a refused write rather than a wrong row.

ALTER TABLE import_run DROP CONSTRAINT import_run_kind_known;

ALTER TABLE import_run
    ADD CONSTRAINT import_run_kind_known
    CHECK (kind IN ('transactions', 'target_year', 'target_month', 'holiday'));
