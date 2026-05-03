-- name: UpsertNNTPCredentials :one
INSERT INTO nntp_credentials (user_id, username, password_enc, key_version)
VALUES (?, ?, ?, ?)
ON CONFLICT(user_id) DO UPDATE SET
  username     = excluded.username,
  password_enc = excluded.password_enc,
  key_version  = excluded.key_version,
  updated_at   = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetNNTPCredentials :one
SELECT * FROM nntp_credentials WHERE user_id = ?;

-- name: DeleteNNTPCredentials :exec
DELETE FROM nntp_credentials WHERE user_id = ?;

-- name: CreateUsenetFeedState :one
INSERT INTO usenet_feed_state (feed_id, provider, group_name)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetUsenetFeedState :one
SELECT ufs.* FROM usenet_feed_state ufs
JOIN feeds f ON f.id = ufs.feed_id
WHERE ufs.feed_id = ? AND f.user_id = ?;

-- name: ListUsenetFeeds :many
SELECT f.*, ufs.group_name, ufs.provider, ufs.high_water_article_number
FROM feeds f
JOIN usenet_feed_state ufs ON ufs.feed_id = f.id
WHERE f.user_id = ?
ORDER BY f.name;

-- name: UpdateUsenetHighWater :exec
UPDATE usenet_feed_state
SET high_water_article_number = ?, updated_at = CURRENT_TIMESTAMP
WHERE feed_id = ?;

-- name: DeleteUsenetFeedState :exec
DELETE FROM usenet_feed_state WHERE feed_id = ?;

-- name: GetUsenetFeedStateByGroup :one
-- Returns the usenet_feed_state row for a specific provider+group owned by a user.
-- Used for duplicate-subscription checks.
SELECT ufs.* FROM usenet_feed_state ufs
JOIN feeds f ON f.id = ufs.feed_id
WHERE ufs.provider = ? AND ufs.group_name = ? AND f.user_id = ?;

-- name: InsertUsenetArticleMeta :one
INSERT INTO usenet_article_meta (
  article_id, feed_id, message_id, references_header,
  parent_message_id, root_message_id, group_name, article_number
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetUsenetArticleMeta :one
SELECT * FROM usenet_article_meta WHERE article_id = ?;

-- name: GetUsenetArticleMetaByMessageID :one
SELECT * FROM usenet_article_meta
WHERE feed_id = ? AND message_id = ?;

-- name: GetUsenetArticleMetaByArticleNumber :one
SELECT * FROM usenet_article_meta
WHERE feed_id = ? AND article_number = ?;

-- name: ListUsenetArticleMetaByThread :many
SELECT * FROM usenet_article_meta
WHERE root_message_id = ?
ORDER BY article_number ASC;

-- name: DeleteUsenetArticleMeta :exec
DELETE FROM usenet_article_meta WHERE article_id = ?;
