-- 0007 — audit, parameters, notifications, jobs.

CREATE TABLE audit_log (
    audit_id     uuid PRIMARY KEY,
    actor_id     uuid REFERENCES app_user (user_id),
    action       text NOT NULL,
    subject_type text NOT NULL,
    subject_id   uuid,
    company_id   uuid REFERENCES company (company_id),
    before_state jsonb,
    after_state  jsonb,
    reason       text,
    ip_address   inet,
    occurred_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_subject_ix ON audit_log (subject_type, subject_id, occurred_at DESC);
CREATE INDEX audit_log_actor_ix ON audit_log (actor_id, occurred_at DESC);
CREATE INDEX audit_log_time_ix ON audit_log (occurred_at DESC);

-- BR-8.1 / D33: append-only, and kept indefinitely. There is no purge job and
-- adding one reverses a decision rather than tidying up.
CREATE TRIGGER audit_log_append_only
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION refuse_mutation();
COMMENT ON TABLE audit_log IS
    'BR-8.1 append-only, BR-8.5/D33 no retention limit. These are event rows, not transaction rows.';

CREATE TABLE sys_parameters (
    param_key    text PRIMARY KEY,
    param_value  text NOT NULL,
    value_type   text NOT NULL DEFAULT 'string',
    description  text NOT NULL,
    is_secret    boolean NOT NULL DEFAULT false,
    updated_by   uuid REFERENCES app_user (user_id),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT sys_parameters_type_known
        CHECK (value_type IN ('string','int','bool','list'))
);
COMMENT ON TABLE sys_parameters IS
    'BR-1.6: anything the business might change without a deploy. A constant in a handler is a defect.';

CREATE TABLE notification_log (
    notification_id uuid PRIMARY KEY,
    channel      text NOT NULL,
    recipient    text NOT NULL,
    subject      text NOT NULL,
    body         text NOT NULL,
    subject_type text,
    subject_id   uuid,
    status       text NOT NULL DEFAULT 'QUEUED',
    attempts     integer NOT NULL DEFAULT 0,
    last_error   text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    sent_at      timestamptz,
    CONSTRAINT notification_status_known CHECK (status IN ('QUEUED','SENT','FAILED')),
    CONSTRAINT notification_channel_known CHECK (channel IN ('email','whatsapp'))
);
CREATE INDEX notification_log_pending_ix ON notification_log (created_at) WHERE status = 'QUEUED';
COMMENT ON TABLE notification_log IS
    'A failed send must not block an approval: the decision commits, the notification queues here and is retried.';

CREATE TABLE job_run (
    job_run_id  uuid PRIMARY KEY,
    job_name    text NOT NULL,
    business_date date,
    outcome     text NOT NULL,
    affected    integer NOT NULL DEFAULT 0,
    message     text,
    started_at  timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    CONSTRAINT job_run_outcome_known CHECK (outcome IN ('OK','FAILED','RUNNING'))
);
-- The daily job is idempotent: one successful run per job per business date.
CREATE UNIQUE INDEX job_run_daily_uk ON job_run (job_name, business_date)
    WHERE outcome = 'OK' AND business_date IS NOT NULL;
CREATE INDEX job_run_recent_ix ON job_run (job_name, started_at DESC);

CREATE TABLE idempotency_key (
    key          text PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES app_user (user_id),
    request_hash text NOT NULL,
    response_body jsonb,
    status_code  integer,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idempotency_key_created_ix ON idempotency_key (created_at);
