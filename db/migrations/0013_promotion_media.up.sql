-- 0013 — marketing media on a promotion (D53).
--
-- Media lines hang off the VERSION, not the plan. That is the whole of BR-4.7
-- applied to a new field: if they hung off the plan, someone could change what
-- the campaign spends after it was approved without the approval moving, and
-- the signature on the plan would no longer be a signature on the numbers.
--
-- It also partly reverses D29 ("no budget field in phase 1"): a promotion now
-- carries what it costs to run, though still no discount cost and still no
-- enforced limit.

CREATE TABLE promotion_media (
    media_id   uuid PRIMARY KEY,
    version_id uuid NOT NULL REFERENCES promotion_plan_version (version_id) ON DELETE CASCADE,
    line_no    integer NOT NULL,
    media_name text NOT NULL,
    price_idr  bigint NOT NULL,

    -- BR-1.1: whole rupiah, and never negative. A negative line would make the
    -- total a number nobody can explain.
    CONSTRAINT promotion_media_price_non_negative CHECK (price_idr >= 0),
    CONSTRAINT promotion_media_name_present CHECK (length(btrim(media_name)) > 0),
    CONSTRAINT promotion_media_line_positive CHECK (line_no > 0),
    -- One row per line per version: an ordering that repeats is an ordering
    -- that renders differently on every read.
    CONSTRAINT promotion_media_line_uk UNIQUE (version_id, line_no)
);

CREATE INDEX promotion_media_version_ix ON promotion_media (version_id, line_no);

COMMENT ON TABLE promotion_media IS
    'BR-3.8 / D53: marketing media for one PLAN VERSION. Versioned with the plan so approved spend cannot be edited without re-approval (BR-4.7).';
COMMENT ON COLUMN promotion_media.price_idr IS
    'Whole rupiah. The total is SUM(price_idr) computed on read — never stored, so it cannot drift from its lines (the BR-2.6 argument, applied here).';
