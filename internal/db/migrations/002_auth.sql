-- +goose Up

CREATE TABLE IF NOT EXISTS users (
    id             SERIAL PRIMARY KEY,
    google_sub     TEXT        NOT NULL UNIQUE,
    email          TEXT        NOT NULL,
    name           TEXT        NOT NULL,
    picture_url    TEXT        NOT NULL DEFAULT '',
    last_login_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sessions (
    id            SERIAL PRIMARY KEY,
    user_id       INT         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash    TEXT        NOT NULL UNIQUE,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE habits
    ADD COLUMN IF NOT EXISTS user_id INT REFERENCES users(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS habits_user_id_created_at_idx ON habits (user_id, created_at);
CREATE INDEX IF NOT EXISTS sessions_user_id_expires_at_idx ON sessions (user_id, expires_at);

-- +goose Down

DROP INDEX IF EXISTS sessions_user_id_expires_at_idx;
DROP INDEX IF EXISTS habits_user_id_created_at_idx;
ALTER TABLE habits DROP COLUMN IF EXISTS user_id;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
