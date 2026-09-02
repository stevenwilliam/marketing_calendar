-- 0004 — the generic approval engine (BR-4, D25).
--
-- Nothing here names a promotion. subject_type + subject_id is the whole
-- coupling, so a purchase order, a leave request or a price change reuses the
-- engine without a schema change.

CREATE TABLE approval_chain (
    chain_id     uuid PRIMARY KEY,
    company_id   uuid NOT NULL REFERENCES company (company_id),
    subject_type text NOT NULL,
    chain_name   text NOT NULL,
    is_active    boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX approval_chain_active_uk
    ON approval_chain (company_id, subject_type) WHERE is_active;
COMMENT ON COLUMN approval_chain.subject_type IS
    'promotion_plan today; purchase_order, leave_request, price_change tomorrow. The engine never learns what they are.';

CREATE TABLE approval_chain_version (
    version_id     uuid PRIMARY KEY,
    chain_id       uuid NOT NULL REFERENCES approval_chain (chain_id) ON DELETE CASCADE,
    version_no     integer NOT NULL,
    effective_from timestamptz NOT NULL DEFAULT now(),
    created_by     uuid REFERENCES app_user (user_id),
    CONSTRAINT approval_chain_version_uk UNIQUE (chain_id, version_no),
    CONSTRAINT approval_chain_version_no_positive CHECK (version_no > 0)
);

CREATE TABLE approval_step (
    step_id      uuid PRIMARY KEY,
    version_id   uuid NOT NULL REFERENCES approval_chain_version (version_id) ON DELETE CASCADE,
    step_no      integer NOT NULL,
    step_name    text NOT NULL,
    satisfaction text NOT NULL,
    CONSTRAINT approval_step_satisfaction_known
        CHECK (satisfaction IN ('ANY_OF', 'ALL_OF')),
    CONSTRAINT approval_step_no_positive CHECK (step_no > 0),
    CONSTRAINT approval_step_order_uk UNIQUE (version_id, step_no)
);

CREATE TABLE approval_step_role (
    step_id uuid NOT NULL REFERENCES approval_step (step_id) ON DELETE CASCADE,
    role_id uuid NOT NULL REFERENCES role (role_id),
    PRIMARY KEY (step_id, role_id)
);
COMMENT ON TABLE approval_step_role IS
    'BR-4.2: satisfaction = ANY_OF with two roles is how "Role_3 OR Role_4" is stored.';

CREATE TABLE approval_instance (
    instance_id     uuid PRIMARY KEY,
    -- BR-4.3 as a foreign key rather than as application discipline: an
    -- instance points at a VERSION, so an administrator rewriting the chain
    -- cannot disturb a plan already in flight.
    version_id      uuid NOT NULL REFERENCES approval_chain_version (version_id),
    company_id      uuid NOT NULL REFERENCES company (company_id),
    subject_type    text NOT NULL,
    subject_id      uuid NOT NULL,
    created_by      uuid NOT NULL REFERENCES app_user (user_id),
    current_step_no integer NOT NULL DEFAULT 1,
    status          text NOT NULL DEFAULT 'PENDING',
    opened_at       timestamptz NOT NULL DEFAULT now(),
    closed_at       timestamptz,
    CONSTRAINT approval_instance_status_known CHECK (
        status IN ('PENDING','APPROVED','REJECTED','CANCELLED','FORCE_RELEASED')),
    CONSTRAINT approval_instance_closed_shape CHECK (
        (status = 'PENDING' AND closed_at IS NULL) OR
        (status <> 'PENDING' AND closed_at IS NOT NULL))
);
CREATE INDEX approval_instance_subject_ix ON approval_instance (subject_type, subject_id);
-- The approver's queue.
CREATE INDEX approval_instance_queue_ix
    ON approval_instance (company_id, current_step_no) WHERE status = 'PENDING';

CREATE TABLE approval_event (
    event_id    uuid PRIMARY KEY,
    instance_id uuid NOT NULL REFERENCES approval_instance (instance_id),
    step_no     integer NOT NULL,
    action      text NOT NULL,
    -- NULL when the actor is the scheduler (BR-4.6). The auto-cancel job must
    -- not be attributed to a person.
    actor_id    uuid REFERENCES app_user (user_id),
    reason      text,
    ip_address  inet,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT approval_event_action_known CHECK (
        action IN ('APPROVE','REJECT','FORCE_RELEASE','AUTO_CANCEL','REVIVE','REOPEN')),
    -- BR-4.8 / BR-4.5 / D27: a bypass or a rejection with no recorded
    -- justification is the control most likely to be questioned in a review.
    CONSTRAINT approval_event_reason_required CHECK (
        action NOT IN ('FORCE_RELEASE','REVIVE','REJECT')
        OR length(btrim(coalesce(reason, ''))) > 0),
    -- The scheduler is the ONLY actor that may be absent.
    CONSTRAINT approval_event_actor_required CHECK (
        action = 'AUTO_CANCEL' OR actor_id IS NOT NULL)
);
CREATE INDEX approval_event_instance_ix ON approval_event (instance_id, occurred_at);

-- BR-4.10: one decision per approver per step. The unique index is the second
-- half of the concurrency control; SELECT ... FOR UPDATE is the first.
CREATE UNIQUE INDEX approval_event_one_per_actor_step
    ON approval_event (instance_id, step_no, actor_id)
    WHERE action = 'APPROVE' AND actor_id IS NOT NULL;

-- BR-4.9: no undo. A mistake is corrected by another event.
CREATE TRIGGER approval_event_append_only
    BEFORE UPDATE OR DELETE ON approval_event
    FOR EACH ROW EXECUTE FUNCTION refuse_mutation();
