-- name: UpsertNNTPCredentials :one
INSERT INTO nntp_credentials (user_id, username, password_enc)
VALUES (?, ?, ?)
ON CONFLICT(user_id) DO UPDATE SET
  username     = excluded.username,
  password_enc = excluded.password_enc,
  updated_at   = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetNNTPCredentials :one
SELECT * FROM nntp_credentials WHERE user_id = ?;

-- name: DeleteNNTPCredentials :exec
DELETE FROM nntp_credentials WHERE user_id = ?;
