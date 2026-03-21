-- name: CreateEntry :exec
INSERT INTO entries (habit_id, day)
VALUES ($1, $2);

-- name: DeleteEntry :exec
DELETE FROM entries WHERE habit_id = $1 AND day = $2;

-- name: EntryExists :one
SELECT EXISTS(
    SELECT 1 FROM entries WHERE habit_id = $1 AND day = $2
) AS exists;

-- name: TodayPoints :one
SELECT COALESCE(SUM(h.points), 0)::int AS total
FROM entries e
JOIN habits h ON h.id = e.habit_id
WHERE e.day = $1;
