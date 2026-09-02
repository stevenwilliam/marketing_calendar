ALTER TABLE promotion_plan DROP CONSTRAINT IF EXISTS promotion_plan_current_version_fk;
DROP TABLE IF EXISTS promotion_plan_version;
DROP TABLE IF EXISTS promotion_plan;
DROP TABLE IF EXISTS sales_target;
