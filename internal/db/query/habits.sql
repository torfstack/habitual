-- name: CreateHabit :one
INSERT INTO habits (name, description)
VALUES ($1, $2)
RETURNING id, name, description, created_at;

-- name: ListHabits :many
SELECT id, name, description, created_at
FROM habits
ORDER BY created_at ASC;

-- name: DeleteHabit :exec
DELETE FROM habits WHERE id = $1;
