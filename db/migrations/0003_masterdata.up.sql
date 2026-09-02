-- 0003 — master data: companies, sites, site groups, holidays.
--
-- The load-bearing constraint here is D31/BR-1.3: a site group's members must
-- all belong to the group's own brand. It is enforced with COMPOSITE FOREIGN
-- KEYS, not a trigger and not an application check, because the row can then
-- only exist if the group and the site agree on the company. A CHECK cannot
-- see two other tables, and a trigger runs only as long as nobody disables it.

CREATE TABLE company (
    company_id   uuid PRIMARY KEY,
    company_code text NOT NULL UNIQUE,
    company_name text NOT NULL,
    is_active    boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE site (
    site_id    uuid PRIMARY KEY,
    company_id uuid NOT NULL REFERENCES company (company_id),
    site_code  text NOT NULL,
    site_name  text NOT NULL,
    site_type  text NOT NULL,
    is_active  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT site_type_known
        CHECK (site_type IN ('coffee_shop', 'restaurant', 'catering')),
    -- BR-1.5: uniqueness is per company, never global.
    CONSTRAINT site_code_uk UNIQUE (company_id, site_code),
    -- The composite target D31 needs. Redundant with the primary key by
    -- itself; it exists so site_group_member can reference (site_id, company_id).
    CONSTRAINT site_id_company_uk UNIQUE (site_id, company_id)
);
CREATE INDEX site_company_ix ON site (company_id) WHERE is_active;

CREATE TABLE site_group (
    site_group_id   uuid PRIMARY KEY,
    company_id      uuid NOT NULL REFERENCES company (company_id),
    site_group_name text NOT NULL,
    is_system       boolean NOT NULL DEFAULT false,
    auto_for_site_id uuid REFERENCES site (site_id),
    is_active       boolean NOT NULL DEFAULT true,
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT site_group_system_shape CHECK (
        (is_system AND auto_for_site_id IS NOT NULL) OR
        (NOT is_system AND auto_for_site_id IS NULL)),
    CONSTRAINT site_group_id_company_uk UNIQUE (site_group_id, company_id)
);
-- BR-1.3: exactly one auto-created group per site.
CREATE UNIQUE INDEX site_group_auto_uk ON site_group (auto_for_site_id) WHERE is_system;
CREATE INDEX site_group_company_ix ON site_group (company_id) WHERE is_active;

CREATE TABLE site_group_member (
    site_group_id uuid NOT NULL,
    site_id       uuid NOT NULL,
    -- Deliberately denormalised (D31). A denormalised column that a foreign
    -- key keeps honest is not duplication; it IS the constraint.
    company_id    uuid NOT NULL,
    PRIMARY KEY (site_group_id, site_id),
    CONSTRAINT sgm_group_fk FOREIGN KEY (site_group_id, company_id)
        REFERENCES site_group (site_group_id, company_id) ON DELETE CASCADE,
    CONSTRAINT sgm_site_fk FOREIGN KEY (site_id, company_id)
        REFERENCES site (site_id, company_id)
);
CREATE INDEX sgm_site_ix ON site_group_member (site_id);
COMMENT ON TABLE site_group_member IS
    'BR-1.3 / D31: a group is restricted to one brand. Both FKs carry company_id, so a cross-brand row cannot exist.';

CREATE TABLE holiday (
    holiday_id   uuid PRIMARY KEY,
    holiday_date date NOT NULL,
    holiday_name text NOT NULL,
    country      text NOT NULL DEFAULT 'ID',
    is_active    boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT holiday_uk UNIQUE (country, holiday_date)
);
-- BR-3.3: the working-day arithmetic reads this on every lead-time check.
CREATE INDEX holiday_date_ix ON holiday (holiday_date) WHERE is_active;
COMMENT ON TABLE holiday IS
    'D22: administrator-maintained. Weekday arithmetic alone gives the wrong lead time around Idul Fitri, Christmas and Nyepi.';

-- user_role lives here because it references company.
-- D37: company_id is NOT NULL. There is no wildcard row meaning "every
-- company" — an implicit superset grows silently when a fourth brand is
-- inserted, granting access nobody decided on.
CREATE TABLE user_role (
    user_id    uuid NOT NULL REFERENCES app_user (user_id) ON DELETE CASCADE,
    role_id    uuid NOT NULL REFERENCES role (role_id),
    company_id uuid NOT NULL REFERENCES company (company_id),
    granted_at timestamptz NOT NULL DEFAULT now(),
    granted_by uuid REFERENCES app_user (user_id),
    PRIMARY KEY (user_id, role_id, company_id)
);
CREATE INDEX user_role_user_ix ON user_role (user_id);
CREATE INDEX user_role_role_company_ix ON user_role (role_id, company_id);
COMMENT ON TABLE user_role IS
    'BR-5.4 / D37: a user is assigned one or more companies EXPLICITLY. Three rows is what "sees all three brands" looks like.';

-- Site scoping (BR-5.5), optional per grant.
CREATE TABLE user_site_scope (
    user_id uuid NOT NULL REFERENCES app_user (user_id) ON DELETE CASCADE,
    site_id uuid NOT NULL REFERENCES site (site_id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, site_id)
);
