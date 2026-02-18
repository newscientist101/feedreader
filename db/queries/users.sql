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

-- name: GetUserIDByNewsletterToken :one
SELECT user_id FROM user_settings WHERE key = 'newsletter_token' AND value = ?;

-- name: GetNewsletterToken :one
SELECT value FROM user_settings WHERE user_id = ? AND key = 'newsletter_token';

-- name: SetNewsletterToken :exec
INSERT INTO user_settings (user_id, key, value) VALUES (?, 'newsletter_token', ?)
ON CONFLICT(user_id, key) DO UPDATE SET value = excluded.value;
