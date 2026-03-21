-- +goose Up

CREATE TABLE IF NOT EXISTS habits (
    id          SERIAL PRIMARY KEY,
    name        TEXT        NOT NULL,
    description TEXT,
    target      INT         NOT NULL DEFAULT 1,
    period      TEXT        NOT NULL DEFAULT 'day' CHECK (period IN ('day', 'week', 'month')),
    deleted_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS entries (
    id         SERIAL PRIMARY KEY,
    habit_id   INT         NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    day        DATE        NOT NULL DEFAULT CURRENT_DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (habit_id, day)
);

-- +goose Down

DROP TABLE IF EXISTS entries;
DROP TABLE IF EXISTS habits;
