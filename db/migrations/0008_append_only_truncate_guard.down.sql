DROP TRIGGER IF EXISTS import_rejection_no_truncate ON import_rejection;
DROP TRIGGER IF EXISTS import_run_no_truncate ON import_run;
DROP TRIGGER IF EXISTS approval_event_no_truncate ON approval_event;
DROP TRIGGER IF EXISTS audit_log_no_truncate ON audit_log;
DROP FUNCTION IF EXISTS refuse_truncate();
