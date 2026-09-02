-- 0008 — close the TRUNCATE hole in the append-only guarantee.
--
-- refuse_mutation() is a FOR EACH ROW trigger, so it fires on UPDATE and
-- DELETE and NOT on TRUNCATE — which is a statement-level operation that
-- removes every row without visiting any of them. Verified against the live
-- database: `TRUNCATE audit_log` emptied the table with the row trigger
-- installed and raised nothing.
--
-- BR-8.1 says these tables take no updates and no deletes. A guarantee that a
-- single statement can erase is not a guarantee, so the statement-level
-- trigger below refuses TRUNCATE too.

CREATE OR REPLACE FUNCTION refuse_truncate() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'tabel % bersifat append-only: TRUNCATE ditolak',
        TG_TABLE_NAME USING ERRCODE = 'restrict_violation';
END; $$;

COMMENT ON FUNCTION refuse_truncate() IS
    'BR-8.1: TRUNCATE is statement-level and never reaches a FOR EACH ROW trigger.';

CREATE TRIGGER audit_log_no_truncate
    BEFORE TRUNCATE ON audit_log
    FOR EACH STATEMENT EXECUTE FUNCTION refuse_truncate();

CREATE TRIGGER approval_event_no_truncate
    BEFORE TRUNCATE ON approval_event
    FOR EACH STATEMENT EXECUTE FUNCTION refuse_truncate();

CREATE TRIGGER import_run_no_truncate
    BEFORE TRUNCATE ON import_run
    FOR EACH STATEMENT EXECUTE FUNCTION refuse_truncate();

-- import_rejection is the evidence behind a rejected row (BR-6.5); losing it
-- loses the reason a line never became revenue.
CREATE TRIGGER import_rejection_no_truncate
    BEFORE TRUNCATE ON import_rejection
    FOR EACH STATEMENT EXECUTE FUNCTION refuse_truncate();
