-- 0006 — the transaction fact table and the importer's records.

CREATE TABLE import_run (
    import_run_id  uuid PRIMARY KEY,
    file_name      text NOT NULL,
    file_checksum  text NOT NULL,
    rows_read      integer NOT NULL DEFAULT 0,
    rows_inserted  integer NOT NULL DEFAULT 0,
    rows_skipped   integer NOT NULL DEFAULT 0,
    rows_rejected  integer NOT NULL DEFAULT 0,
    trailer_rows   integer,
    trailer_total_idr bigint,
    outcome        text NOT NULL,
    message        text,
    actor_id       uuid REFERENCES app_user (user_id),
    started_at     timestamptz NOT NULL DEFAULT now(),
    finished_at    timestamptz,
    CONSTRAINT import_run_outcome_known CHECK (outcome IN ('OK','PARTIAL','FAILED','SKIPPED')),
    CONSTRAINT import_run_counts_sane CHECK (
        rows_read >= 0 AND rows_inserted >= 0 AND rows_skipped >= 0 AND rows_rejected >= 0)
);
-- BR-6.3/6.4: identity is the CHECKSUM, not the filename, so renaming a file
-- does not let it in twice.
CREATE UNIQUE INDEX import_run_checksum_uk ON import_run (file_checksum)
    WHERE outcome IN ('OK','PARTIAL');
CREATE TRIGGER import_run_append_only
    BEFORE UPDATE OR DELETE ON import_run
    FOR EACH ROW EXECUTE FUNCTION refuse_mutation();

CREATE TABLE history_txn (
    txn_id           uuid PRIMARY KEY,
    company_id       uuid NOT NULL REFERENCES company (company_id),
    site_id          uuid NOT NULL REFERENCES site (site_id),
    business_date    date NOT NULL,
    pos_receipt_no   text NOT NULL,
    sales_type       text NOT NULL,
    promo_id         uuid REFERENCES promotion_plan (plan_id),
    order_mode       text NOT NULL,
    gross_amount_idr bigint NOT NULL,
    import_run_id    uuid REFERENCES import_run (import_run_id),
    created_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT history_txn_amount_non_negative CHECK (gross_amount_idr >= 0),
    CONSTRAINT history_txn_sales_type_known CHECK (sales_type IN ('normal','promo')),
    CONSTRAINT history_txn_order_mode_known CHECK (order_mode IN ('dine_in','take_away')),
    -- BR-7.6: this is what stops a `normal` row carrying a promotion id and
    -- being double-counted in the promo report.
    CONSTRAINT history_txn_promo_consistency CHECK (
        (sales_type = 'promo'  AND promo_id IS NOT NULL) OR
        (sales_type = 'normal' AND promo_id IS NULL))
);

-- BR-6.3: re-importing the same file inserts nothing.
CREATE UNIQUE INDEX history_txn_receipt_uk
    ON history_txn (site_id, business_date, pos_receipt_no);
CREATE INDEX history_txn_report_ix ON history_txn (company_id, business_date);
CREATE INDEX history_txn_site_date_ix ON history_txn (site_id, business_date);
CREATE INDEX history_txn_promo_ix ON history_txn (promo_id) WHERE promo_id IS NOT NULL;

COMMENT ON TABLE history_txn IS
    'BR-6.2 / D6: one row per receipt. Receipt count is count(*), which is why the grain cannot be changed once history is loaded.';

CREATE TABLE import_rejection (
    rejection_id  uuid PRIMARY KEY,
    import_run_id uuid NOT NULL REFERENCES import_run (import_run_id) ON DELETE CASCADE,
    line_no       integer NOT NULL,
    reason        text NOT NULL,
    original_line text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX import_rejection_run_ix ON import_rejection (import_run_id);
COMMENT ON TABLE import_rejection IS
    'BR-6.5: a rejected row never silently disappears. The reason and the original line are both kept.';
