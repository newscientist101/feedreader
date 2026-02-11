-- name: GetUserByExternalID :one
SELECT * FROM users WHERE external_id = ?;

-- name: CreateUser :one
INSERT INTO users (external_id, email)
VALUES (?, ?)
RETURNING *;

-- name: UpdateUserLastSeen :exec
UPDATE users SET last_seen_at = CURRENT_TIMESTAMP, email = ? WHERE id = ?;

-- name: GetOrCreateUser :one
INSERT INTO users (external_id, email)
VALUES (?, ?)
ON CONFLICT(external_id) DO UPDATE SET 
  last_seen_at = CURRENT_TIMESTAMP,
  email = excluded.email
RETURNING *;
