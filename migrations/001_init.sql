CREATE TABLE IF NOT EXISTS habits (
    id         SERIAL PRIMARY KEY,
    name       TEXT        NOT NULL,
    description TEXT,
    points     INT         NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS entries (
    id         SERIAL PRIMARY KEY,
    habit_id   INT         NOT NULL REFERENCES habits(id) ON DELETE CASCADE,
    day        DATE        NOT NULL DEFAULT CURRENT_DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (habit_id, day)
);
