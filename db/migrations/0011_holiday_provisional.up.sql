-- 0011 — a holiday can be provisional.
--
-- The holiday calendar drives the promotion lead time (BR-3.3). Indonesian
-- public holidays fall into three groups and only the first is knowable in
-- advance:
--
--   fixed    1 Jan, 1 May, 1 Jun, 17 Aug, 25 Dec — certain, every year
--   derived  Good Friday and Ascension — computable exactly from Easter
--   decreed  Idul Fitri, Idul Adha, Nyepi, Waisak, Imlek, Maulid, Isra
--            Mikraj, and every cuti bersama — set by joint ministerial decree,
--            usually the year before, and NOT reliably computable
--
-- Seeding the third group without saying so would hand the lead-time
-- calculation a guess dressed as a fact. is_provisional says which dates still
-- need confirming against the decree, and the UI shows it.

ALTER TABLE holiday
    ADD COLUMN is_provisional boolean NOT NULL DEFAULT false;

COMMENT ON COLUMN holiday.is_provisional IS
    'true = an estimate that has not been confirmed against the official decree. Drives the lead time all the same, so it is shown in the UI rather than buried.';

CREATE INDEX holiday_provisional_ix ON holiday (holiday_date)
    WHERE is_active AND is_provisional;
