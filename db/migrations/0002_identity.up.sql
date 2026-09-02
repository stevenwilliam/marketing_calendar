-- 0002 — identity: users, roles, permissions, sessions, TOTP.

CREATE TABLE app_user (
    user_id        uuid PRIMARY KEY,
    email          text NOT NULL,
    full_name      text NOT NULL,
    password_hash  text NOT NULL,
    is_active      boolean NOT NULL DEFAULT true,
    locked_until   timestamptz,
    failed_logins  integer NOT NULL DEFAULT 0,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    -- 12-security.md §3: a CHECK refuses anything that is not an argon2id
    -- hash, so a bcrypt or plaintext value cannot be loaded by a fixture and
    -- then quietly fail to match like a wrong password.
    CONSTRAINT app_user_password_is_argon2id
        CHECK (password_hash LIKE '$argon2id$%'),
    CONSTRAINT app_user_failed_logins_sane CHECK (failed_logins >= 0)
);
CREATE UNIQUE INDEX app_user_email_uk ON app_user (lower(email));

CREATE TABLE user_totp (
    user_id      uuid PRIMARY KEY REFERENCES app_user (user_id) ON DELETE CASCADE,
    secret       text NOT NULL,
    confirmed_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE user_totp IS
    'BR-5.3 / D18: TOTP is mandatory. A user without confirmed_at reaches only the enrolment flow.';

CREATE TABLE permission (
    permission_code text PRIMARY KEY,
    description     text NOT NULL
);

CREATE TABLE role (
    role_id     uuid PRIMARY KEY,
    role_code   text NOT NULL UNIQUE,
    label_id    text NOT NULL,
    label_en    text NOT NULL,
    is_system   boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE role IS 'BR-5.7 / D28: the group''s real roles. Roles are data, not code.';

CREATE TABLE role_permission (
    role_id         uuid NOT NULL REFERENCES role (role_id) ON DELETE CASCADE,
    permission_code text NOT NULL REFERENCES permission (permission_code) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_code)
);

CREATE TABLE refresh_token (
    token_id    uuid PRIMARY KEY,
    user_id     uuid NOT NULL REFERENCES app_user (user_id) ON DELETE CASCADE,
    family_id   uuid NOT NULL,
    token_hash  text NOT NULL UNIQUE,
    issued_at   timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz,
    revoked_at  timestamptz,
    user_agent  text,
    ip_address  inet
);
-- BR-5.6: re-presenting a rotated token revokes the whole family, so the
-- family is indexed as the unit of revocation.
CREATE INDEX refresh_token_family_ix ON refresh_token (family_id);
CREATE INDEX refresh_token_user_ix ON refresh_token (user_id, expires_at DESC);
COMMENT ON COLUMN refresh_token.token_hash IS
    'SHA-256. The token itself is never stored: a database read must not hand over live sessions.';

CREATE TABLE jti_denylist (
    jti        text PRIMARY KEY,
    expires_at timestamptz NOT NULL
);
COMMENT ON TABLE jti_denylist IS
    'Logout revokes the access token for its remaining life. Rows are purged once expired.';
CREATE INDEX jti_denylist_expiry_ix ON jti_denylist (expires_at);
