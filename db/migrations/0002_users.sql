-- Users, sessions and the column types that decide what a database dump is
-- worth to an attacker.
--
-- Three different secrets, three different treatments:
--   password        one-way hash (argon2id)          — the server never needs it back
--   security answer one-way hash                     — a bearer secret with low entropy
--   TOTP secret     encrypted (AES-256-GCM)          — it must be reproduced to check a code

CREATE TYPE user_role AS ENUM ('administrator', 'network-engineer', 'noc-operator', 'viewer');

CREATE TABLE users (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username             TEXT NOT NULL,
    full_name            TEXT NOT NULL,
    email                TEXT NOT NULL,
    phone                TEXT,
    role                 user_role NOT NULL DEFAULT 'viewer',
    -- PHC string: $argon2id$v=19$m=...,t=...,p=...$salt$hash
    password_hash        TEXT NOT NULL,
    security_question    TEXT,
    security_answer_hash TEXT,
    totp_secret_enc      BYTEA,
    totp_enabled         BOOLEAN NOT NULL DEFAULT FALSE,
    disabled             BOOLEAN NOT NULL DEFAULT FALSE,
    failed_attempts      INT NOT NULL DEFAULT 0,
    locked_until         TIMESTAMPTZ,
    last_login_at        TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Case-insensitive uniqueness without the citext extension.
CREATE UNIQUE INDEX users_username_lower_idx ON users (lower(username));
CREATE UNIQUE INDEX users_email_lower_idx ON users (lower(email));

CREATE TABLE sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- SHA-256 of the cookie value, never the cookie itself: a dump of this
    -- table cannot be replayed as a login.
    token_hash BYTEA NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    ip         INET,
    user_agent TEXT
);

CREATE INDEX sessions_user_idx   ON sessions (user_id);
CREATE INDEX sessions_expiry_idx ON sessions (expires_at);

-- Password changes and role changes are auditable events, so the audit table
-- gets a reference to the actor as a real user rather than free text.
ALTER TABLE audit_log ADD COLUMN actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL;
