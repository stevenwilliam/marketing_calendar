-- 0010 — the importer carries more than one kind of file.
--
-- Transactions, yearly targets and monthly targets all arrive as pipe-delimited
-- CSV through the same drop directory and the same reconciliation screen. The
-- run row has to say which kind it was, or the screen cannot tell a night with
-- no sales from a night that loaded a target file.

ALTER TABLE import_run
    ADD COLUMN kind text NOT NULL DEFAULT 'transactions';

ALTER TABLE import_run
    ADD CONSTRAINT import_run_kind_known
    CHECK (kind IN ('transactions', 'target_year', 'target_month'));

COMMENT ON COLUMN import_run.kind IS
    'Detected from the header, never from the filename: a file renamed by hand must not change how it is parsed.';

CREATE INDEX import_run_kind_ix ON import_run (kind, started_at DESC);

-- A target import writes sales_target rows, which are NOT append-only and are
-- upserted by the (site, period, sales_type) unique index. The run row records
-- how many were written; the rows themselves carry updated_by as usual.
