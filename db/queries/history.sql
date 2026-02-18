-- name: AddToHistory :exec
INSERT INTO history_articles (user_id, article_id)
VALUES (?, ?)
ON CONFLICT(user_id, article_id) DO UPDATE SET viewed_at = CURRENT_TIMESTAMP;

-- name: ListHistoryArticles :many
SELECT a.id, a.feed_id, a.guid, a.title, a.url, a.author, a.content, a.summary,
       a.image_url, a.published_at, a.fetched_at, a.is_read, a.is_starred,
       f.name as feed_name, h.viewed_at
FROM history_articles h
JOIN articles a ON h.article_id = a.id
JOIN feeds f ON a.feed_id = f.id
WHERE h.user_id = ?
ORDER BY h.viewed_at DESC
LIMIT ? OFFSET ?;

-- name: GetHistoryCount :one
SELECT COUNT(*) FROM history_articles WHERE user_id = ?;
