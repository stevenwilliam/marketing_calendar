-- 0005 — targets and promotion plans.

CREATE TABLE sales_target (
    target_id         uuid PRIMARY KEY,
    company_id        uuid NOT NULL REFERENCES company (company_id),
    site_id           uuid NOT NULL REFERENCES site (site_id),
    period_kind       text NOT NULL,
    period_year       integer NOT NULL,
    period_month      integer,
    sales_type        text NOT NULL,
    target_amount_idr bigint NOT NULL,
    updated_by        uuid REFERENCES app_user (user_id),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    created_at        timestamptz NOT NULL DEFAULT now(),
    -- BR-2.4: zero is meaningful (a site closed that month); negative is not.
    CONSTRAINT sales_target_amount_non_negative CHECK (target_amount_idr >= 0),
    CONSTRAINT sales_target_sales_type_known CHECK (sales_type IN ('normal','promo')),
    CONSTRAINT sales_target_period_shape CHECK (
        (period_kind = 'YEAR'  AND period_month IS NULL) OR
        (period_kind = 'MONTH' AND period_month BETWEEN 1 AND 12)),
    CONSTRAINT sales_target_year_sane CHECK (period_year BETWEEN 2000 AND 2999)
);

-- BR-2.1/2.2: one target per site, period and sales type.
CREATE UNIQUE INDEX sales_target_uk
    ON sales_target (site_id, period_kind, period_year, COALESCE(period_month, 0), sales_type);
CREATE INDEX sales_target_lookup_ix ON sales_target (company_id, period_year, period_month);

-- There is DELIBERATELY no constraint that the twelve month targets sum to the
-- year target. BR-2.3 says over or under is valid and must not be blocked. A
-- future reader will be tempted to add one: the absence is the rule, and
-- target_test.go::TestMonthsNeedNotSumToYear is the guard.
COMMENT ON TABLE sales_target IS
    'BR-2.3: the months need not sum to the year. Do not add a constraint that they must.';

CREATE TABLE promotion_plan (
    plan_id             uuid PRIMARY KEY,
    company_id          uuid NOT NULL REFERENCES company (company_id),
    plan_code           text NOT NULL UNIQUE,
    site_group_id       uuid NOT NULL,
    current_version_id  uuid,
    status              text NOT NULL DEFAULT 'DRAFT',
    approval_instance_id uuid REFERENCES approval_instance (instance_id),
    force_released      boolean NOT NULL DEFAULT false,
    created_by          uuid NOT NULL REFERENCES app_user (user_id),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT promotion_plan_status_known CHECK (
        status IN ('DRAFT','PENDING','RELEASED','REJECTED','CANCELLED')),
    -- BR-3.7 / D31: the plan and its site group are the same brand. The
    -- composite FK makes a cross-brand plan unrepresentable.
    CONSTRAINT promotion_plan_group_fk FOREIGN KEY (site_group_id, company_id)
        REFERENCES site_group (site_group_id, company_id),
    -- force_released is only meaningful once the plan is out.
    CONSTRAINT promotion_plan_force_shape CHECK (
        NOT force_released OR status = 'RELEASED')
);
CREATE INDEX promotion_plan_queue_ix ON promotion_plan (company_id, status);
CREATE INDEX promotion_plan_group_ix ON promotion_plan (site_group_id);

CREATE TABLE promotion_plan_version (
    version_id           uuid PRIMARY KEY,
    plan_id              uuid NOT NULL REFERENCES promotion_plan (plan_id) ON DELETE CASCADE,
    version_no           integer NOT NULL,
    promo_name           text NOT NULL,
    start_date           date NOT NULL,
    end_date             date NOT NULL,
    target_sales_idr     bigint NOT NULL,
    target_receipt_count integer NOT NULL,
    order_mode           text NOT NULL,
    promo_rule           text NOT NULL,
    overlap_acknowledged boolean NOT NULL DEFAULT false,
    lead_time_overridden boolean NOT NULL DEFAULT false,
    lead_time_override_reason text,
    created_by           uuid NOT NULL REFERENCES app_user (user_id),
    created_at           timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT promo_period_ordered CHECK (end_date >= start_date),          -- BR-3.2
    CONSTRAINT promo_targets_non_negative
        CHECK (target_sales_idr >= 0 AND target_receipt_count >= 0),         -- BR-3.5
    CONSTRAINT promo_order_mode_known CHECK (order_mode IN ('dine_in','take_away')), -- BR-3.4
    -- An override with no reason is not an override (BR-3.3, BR-4.8).
    CONSTRAINT promo_override_needs_reason CHECK (
        NOT lead_time_overridden
        OR length(btrim(coalesce(lead_time_override_reason, ''))) > 0),
    CONSTRAINT promo_version_uk UNIQUE (plan_id, version_no),
    CONSTRAINT promo_version_no_positive CHECK (version_no > 0)
);
CREATE INDEX promotion_plan_version_current_ix
    ON promotion_plan_version (plan_id, version_no DESC);
CREATE INDEX promotion_plan_version_dates_ix ON promotion_plan_version (start_date, end_date);

COMMENT ON TABLE promotion_plan_version IS
    'BR-4.7 / D12 made structural: the substantive fields live here, and an approved version is simply never written to again.';

ALTER TABLE promotion_plan
    ADD CONSTRAINT promotion_plan_current_version_fk
    FOREIGN KEY (current_version_id) REFERENCES promotion_plan_version (version_id)
    DEFERRABLE INITIALLY DEFERRED;
