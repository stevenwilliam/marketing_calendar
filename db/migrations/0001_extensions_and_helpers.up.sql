-- 0001 — extensions and the append-only guard.
--
-- refuse_mutation() is the trigger behind BR-8.1: history tables take no
-- UPDATE and no DELETE. It is a trigger rather than a convention because a
-- convention is bypassed by the next person writing a repair script at 2am.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE OR REPLACE FUNCTION refuse_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'tabel % bersifat append-only: % ditolak',
        TG_TABLE_NAME, TG_OP USING ERRCODE = 'restrict_violation';
END; $$;

COMMENT ON FUNCTION refuse_mutation() IS
    'BR-8.1 / BR-4.9 / BR-6.4: append-only history. No updates, no deletes.';
