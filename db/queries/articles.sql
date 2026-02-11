-- name: CreateArticle :one
INSERT INTO articles (feed_id, guid, title, url, author, content, summary, image_url, published_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(feed_id, guid) DO UPDATE SET
  title = excluded.title,
  url = excluded.url,
  author = excluded.author,
  content = excluded.content,
  summary = excluded.summary,
  image_url = excluded.image_url,
  published_at = excluded.published_at
RETURNING *;

-- name: GetArticle :one
SELECT * FROM articles WHERE id = ?;

-- name: ListArticles :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: ListArticlesByFeed :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.feed_id = ?
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: ListUnreadArticles :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.is_read = 0
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: ListStarredArticles :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.is_starred = 1
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: MarkArticleRead :exec
UPDATE articles SET is_read = 1 WHERE id = ?;

-- name: MarkArticleUnread :exec
UPDATE articles SET is_read = 0 WHERE id = ?;

-- name: ToggleArticleStar :exec
UPDATE articles SET is_starred = NOT is_starred WHERE id = ?;

-- name: MarkFeedRead :exec
UPDATE articles SET is_read = 1 WHERE feed_id = ?;

-- name: MarkAllRead :exec
UPDATE articles SET is_read = 1;

-- name: GetUnreadCount :one
SELECT COUNT(*) as count FROM articles WHERE is_read = 0;

-- name: GetStarredCount :one
SELECT COUNT(*) as count FROM articles WHERE is_starred = 1;

-- name: SearchArticles :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.title LIKE '%' || ? || '%' OR a.content LIKE '%' || ? || '%'
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: ListArticlesByCategory :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
JOIN feed_categories fc ON f.id = fc.feed_id
WHERE fc.category_id = ?
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: ListUnreadArticlesByCategory :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
JOIN feed_categories fc ON f.id = fc.feed_id
WHERE fc.category_id = ? AND a.is_read = 0
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: MarkCategoryRead :exec
UPDATE articles SET is_read = 1 
WHERE feed_id IN (
  SELECT feed_id FROM feed_categories WHERE category_id = ?
);
