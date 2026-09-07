DROP INDEX IF EXISTS holiday_provisional_ix;
ALTER TABLE holiday DROP COLUMN IF EXISTS is_provisional;
