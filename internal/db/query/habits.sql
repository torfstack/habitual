-- name: CreateHabit :one
INSERT INTO habits (name, description, points)
VALUES ($1, $2, $3)
RETURNING id, name, description, points, created_at;

-- name: ListHabits :many
SELECT id, name, description, points, created_at
FROM habits
ORDER BY created_at ASC;

-- name: DeleteHabit :exec
DELETE FROM habits WHERE id = $1;
