DROP TABLE IF EXISTS idempotency_key;
DROP TABLE IF EXISTS job_run;
DROP TABLE IF EXISTS notification_log;
DROP TABLE IF EXISTS sys_parameters;
DROP TRIGGER IF EXISTS audit_log_append_only ON audit_log;
DROP TABLE IF EXISTS audit_log;
