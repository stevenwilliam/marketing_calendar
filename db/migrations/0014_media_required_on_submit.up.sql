-- 0014 — a plan entering the approval chain must carry at least one media line
-- (BR-3.8, D54).
--
-- Enforced by a trigger rather than a CHECK because the rule spans two tables
-- and only bites on a STATUS TRANSITION: a DRAFT with no media is legitimate
-- (BR-3.1 — a draft may be incomplete), and the same row becomes invalid the
-- moment it is submitted.
--
-- The application refuses this first, with a message naming the field. This is
-- the second line: it holds for any path into the table, including a repair
-- script at 2am, which is the whole argument for putting invariants in the
-- database rather than only in the code that usually writes to it.

CREATE OR REPLACE FUNCTION require_media_on_submit() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    -- Only the DRAFT/REJECTED -> PENDING transition. Re-saving a plan that is
    -- already PENDING, or moving it to RELEASED, must not be re-validated
    -- against a rule that was checked when it entered the chain.
    IF NEW.status = 'PENDING' AND (OLD.status IS DISTINCT FROM 'PENDING') THEN
        IF NEW.current_version_id IS NULL
           OR NOT EXISTS (SELECT 1 FROM promotion_media
                           WHERE version_id = NEW.current_version_id) THEN
            RAISE EXCEPTION
                'rencana % tidak dapat diajukan tanpa media pemasaran', NEW.plan_code
                USING ERRCODE = 'check_violation';
        END IF;
    END IF;
    RETURN NEW;
END; $$;

COMMENT ON FUNCTION require_media_on_submit() IS
    'BR-3.8 / D54: at least one media line before a plan enters the approval chain.';

CREATE TRIGGER promotion_plan_media_required
    BEFORE UPDATE ON promotion_plan
    FOR EACH ROW EXECUTE FUNCTION require_media_on_submit();
