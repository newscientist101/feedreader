-- name: AddToQueue :exec
INSERT OR IGNORE INTO queue_articles (user_id, article_id) VALUES (?, ?);

-- name: RemoveFromQueue :exec
DELETE FROM queue_articles WHERE user_id = ? AND article_id = ?;

-- name: IsArticleQueued :one
SELECT COUNT(*) FROM queue_articles WHERE user_id = ? AND article_id = ?;

-- name: ListQueueArticles :many
SELECT a.id, a.feed_id, a.guid, a.title, a.url, a.author, a.content, a.summary,
       a.image_url, a.published_at, a.fetched_at, a.is_read, a.is_starred,
       f.name as feed_name
FROM queue_articles qa
JOIN articles a ON qa.article_id = a.id
JOIN feeds f ON a.feed_id = f.id
WHERE qa.user_id = ?
ORDER BY COALESCE(a.published_at, a.fetched_at) ASC
LIMIT ? OFFSET ?;

-- name: GetQueueCount :one
SELECT COUNT(*) FROM queue_articles WHERE user_id = ?;

-- name: ListQueueArticlesCursor :many
SELECT a.id, a.feed_id, a.guid, a.title, a.url, a.author, a.content, a.summary,
       a.image_url, a.published_at, a.fetched_at, a.is_read, a.is_starred,
       f.name as feed_name
FROM queue_articles qa
JOIN articles a ON qa.article_id = a.id
JOIN feeds f ON a.feed_id = f.id
WHERE qa.user_id = ?
  AND (COALESCE(a.published_at, a.fetched_at) > @after_time
       OR (COALESCE(a.published_at, a.fetched_at) = @after_time_eq AND a.id > @after_id))
ORDER BY COALESCE(a.published_at, a.fetched_at) ASC, a.id ASC
LIMIT ?;
