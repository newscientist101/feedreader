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
SELECT a.* FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.id = ? AND f.user_id = ?;

-- name: GetArticleWithFeed :one
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.id = ? AND f.user_id = ?;

-- name: ListArticles :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE f.user_id = ?
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: ListArticlesByFeed :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.feed_id = ? AND f.user_id = ?
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: ListUnreadArticles :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.is_read = 0 AND f.user_id = ?
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: ListStarredArticles :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.is_starred = 1 AND f.user_id = ?
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: ListStarredArticlesCursor :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.is_starred = 1 AND f.user_id = ?
  AND (COALESCE(a.published_at, a.fetched_at) < @before_time
       OR (COALESCE(a.published_at, a.fetched_at) = @before_time_eq AND a.id < @before_id))
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC, a.id DESC
LIMIT ?;

-- name: ListArticlesCursor :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE f.user_id = ?
  AND (COALESCE(a.published_at, a.fetched_at) < @before_time
       OR (COALESCE(a.published_at, a.fetched_at) = @before_time_eq AND a.id < @before_id))
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC, a.id DESC
LIMIT ?;

-- name: ListArticlesByFeedCursor :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.feed_id = ? AND f.user_id = ?
  AND (COALESCE(a.published_at, a.fetched_at) < @before_time
       OR (COALESCE(a.published_at, a.fetched_at) = @before_time_eq AND a.id < @before_id))
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC, a.id DESC
LIMIT ?;

-- name: ListUnreadArticlesCursor :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.is_read = 0 AND f.user_id = ?
  AND (COALESCE(a.published_at, a.fetched_at) < @before_time
       OR (COALESCE(a.published_at, a.fetched_at) = @before_time_eq AND a.id < @before_id))
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC, a.id DESC
LIMIT ?;

-- name: ListArticlesByCategoryCursor :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
JOIN feed_categories fc ON f.id = fc.feed_id
WHERE fc.category_id = ? AND f.user_id = ?
  AND (COALESCE(a.published_at, a.fetched_at) < @before_time
       OR (COALESCE(a.published_at, a.fetched_at) = @before_time_eq AND a.id < @before_id))
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC, a.id DESC
LIMIT ?;

-- name: ListUnreadArticlesByCategoryCursor :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
JOIN feed_categories fc ON f.id = fc.feed_id
WHERE fc.category_id = ? AND a.is_read = 0 AND f.user_id = ?
  AND (COALESCE(a.published_at, a.fetched_at) < @before_time
       OR (COALESCE(a.published_at, a.fetched_at) = @before_time_eq AND a.id < @before_id))
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC, a.id DESC
LIMIT ?;

-- name: MarkArticleRead :exec
UPDATE articles SET is_read = 1 
WHERE articles.id = ? AND feed_id IN (SELECT feeds.id FROM feeds WHERE feeds.user_id = ?);

-- name: MarkArticleUnread :exec
UPDATE articles SET is_read = 0 
WHERE articles.id = ? AND feed_id IN (SELECT feeds.id FROM feeds WHERE feeds.user_id = ?);

-- name: ToggleArticleStar :exec
UPDATE articles SET is_starred = NOT is_starred 
WHERE articles.id = ? AND feed_id IN (SELECT feeds.id FROM feeds WHERE feeds.user_id = ?);

-- name: MarkFeedRead :exec
UPDATE articles SET is_read = 1 
WHERE feed_id = ? AND feed_id IN (SELECT id FROM feeds WHERE user_id = ?);

-- name: MarkAllRead :exec
UPDATE articles SET is_read = 1 
WHERE feed_id IN (SELECT id FROM feeds WHERE user_id = ?);

-- name: GetUnreadCount :one
SELECT COUNT(*) as count FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.is_read = 0 AND f.user_id = ?;

-- name: GetStarredCount :one
SELECT COUNT(*) as count FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.is_starred = 1 AND f.user_id = ?;

-- name: SearchArticles :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE f.user_id = ? AND (a.title LIKE '%' || ? || '%' OR a.content LIKE '%' || ? || '%')
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: SearchArticlesByFeed :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.feed_id = ? AND f.user_id = ? AND (a.title LIKE '%' || ? || '%' OR a.content LIKE '%' || ? || '%')
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: SearchArticlesByCategory :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
JOIN feed_categories fc ON f.id = fc.feed_id
WHERE fc.category_id = ? AND f.user_id = ? AND (a.title LIKE '%' || ? || '%' OR a.content LIKE '%' || ? || '%')
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: ListArticlesByCategory :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
JOIN feed_categories fc ON f.id = fc.feed_id
WHERE fc.category_id = ? AND f.user_id = ?
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: ListUnreadArticlesByCategory :many
SELECT a.*, f.name as feed_name FROM articles a
JOIN feeds f ON a.feed_id = f.id
JOIN feed_categories fc ON f.id = fc.feed_id
WHERE fc.category_id = ? AND a.is_read = 0 AND f.user_id = ?
ORDER BY COALESCE(a.published_at, a.fetched_at) DESC
LIMIT ? OFFSET ?;

-- name: MarkCategoryRead :exec
UPDATE articles SET is_read = 1 
WHERE feed_id IN (
  SELECT f.id FROM feeds f
  JOIN feed_categories fc ON f.id = fc.feed_id 
  WHERE fc.category_id = ? AND f.user_id = ?
);

-- name: DeleteOldUnstarredArticles :execresult
DELETE FROM articles
WHERE articles.is_starred = 0 
  AND articles.id NOT IN (SELECT article_id FROM queue_articles)
  AND articles.fetched_at < datetime('now', '-' || ? || ' days')
  AND articles.feed_id IN (SELECT id FROM feeds WHERE feeds.user_id = ?);

-- name: CountOldUnstarredArticles :one
SELECT COUNT(*) FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.is_starred = 0 
  AND a.id NOT IN (SELECT article_id FROM queue_articles)
  AND a.fetched_at < datetime('now', '-' || ? || ' days')
  AND f.user_id = ?;

-- name: GetOldestArticleDate :one
SELECT MIN(a.fetched_at) FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.is_starred = 0 AND f.user_id = ?;

-- name: MarkFeedArticlesReadOlderThan :exec
UPDATE articles SET is_read = 1 
WHERE feed_id = ? AND is_read = 0 
  AND COALESCE(published_at, fetched_at) < datetime('now', '-' || ? || ' days')
  AND feed_id IN (SELECT id FROM feeds WHERE user_id = ?);

-- name: MarkCategoryArticlesReadOlderThan :exec
UPDATE articles SET is_read = 1 
WHERE feed_id IN (
    SELECT f.id FROM feeds f
    JOIN feed_categories fc ON f.id = fc.feed_id 
    WHERE fc.category_id = ? AND f.user_id = ?
) AND is_read = 0 
  AND COALESCE(published_at, fetched_at) < datetime('now', '-' || ? || ' days');

-- name: MarkAllArticlesRead :exec
UPDATE articles SET is_read = 1 
WHERE is_read = 0 AND feed_id IN (SELECT id FROM feeds WHERE user_id = ?);

-- name: MarkAllArticlesReadOlderThan :exec
UPDATE articles SET is_read = 1 
WHERE is_read = 0 
  AND COALESCE(published_at, fetched_at) < datetime('now', '-' || ? || ' days')
  AND feed_id IN (SELECT id FROM feeds WHERE user_id = ?);

-- name: DeleteOldUnstarredArticlesGlobal :execresult
-- For background cleanup - deletes across all users
DELETE FROM articles
WHERE is_starred = 0 
  AND id NOT IN (SELECT article_id FROM queue_articles)
  AND fetched_at < datetime('now', '-' || ? || ' days');

-- name: CountOldUnstarredArticlesGlobal :one
SELECT COUNT(*) FROM articles
WHERE is_starred = 0 
  AND id NOT IN (SELECT article_id FROM queue_articles)
  AND fetched_at < datetime('now', '-' || ? || ' days');

-- name: GetOldestArticleDateGlobal :one
SELECT MIN(fetched_at) FROM articles WHERE is_starred = 0;

-- name: ListUnreadArticlesByFeedInternal :many
-- Used internally (no user_id check) to apply exclusion rules after fetch
SELECT id, title, summary, author FROM articles
WHERE feed_id = ? AND is_read = 0;

-- name: MarkArticleReadInternal :exec
-- Mark a single article read without user_id check (for exclusion auto-marking)
UPDATE articles SET is_read = 1 WHERE id = ?;
-- name: ListUnreadArticlesByCategoryInternal :many
-- Used internally to apply exclusion rules when a new rule is created
SELECT a.id, a.title, a.summary, a.author FROM articles a
JOIN feed_categories fc ON a.feed_id = fc.feed_id
WHERE fc.category_id = ? AND a.is_read = 0;
