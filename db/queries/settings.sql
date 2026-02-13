-- name: GetUserSettings :many
SELECT key, value FROM user_settings WHERE user_id = ?;

-- name: GetUserSetting :one
SELECT value FROM user_settings WHERE user_id = ? AND key = ?;

-- name: SetUserSetting :exec
INSERT INTO user_settings (user_id, key, value)
VALUES (?, ?, ?)
ON CONFLICT(user_id, key) DO UPDATE SET value = excluded.value;

-- name: DeleteUserSetting :exec
DELETE FROM user_settings WHERE user_id = ? AND key = ?;
