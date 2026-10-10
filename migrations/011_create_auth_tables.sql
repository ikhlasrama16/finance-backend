BEGIN;

CREATE TABLE users (
    id UUID PRIMARY KEY,
    email VARCHAR(320) NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_email_not_blank CHECK (BTRIM(email) <> '')
);

CREATE UNIQUE INDEX idx_users_email ON users (LOWER(email));

CREATE TABLE user_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_sessions_user ON user_sessions (user_id);
CREATE INDEX idx_user_sessions_expires ON user_sessions (expires_at);

COMMIT;
