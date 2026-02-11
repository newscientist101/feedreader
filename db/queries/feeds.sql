-- name: CreateFeed :one
INSERT INTO feeds (name, url, feed_type, scraper_module, scraper_config, fetch_interval_minutes)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetFeed :one
SELECT * FROM feeds WHERE id = ?;

-- name: GetFeedByURL :one
SELECT * FROM feeds WHERE url = ?;

-- name: ListFeeds :many
SELECT * FROM feeds ORDER BY name;

-- name: ListFeedsToFetch :many
SELECT * FROM feeds 
WHERE last_fetched_at IS NULL 
   OR datetime(last_fetched_at, '+' || fetch_interval_minutes || ' minutes') < datetime('now')
ORDER BY last_fetched_at NULLS FIRST;

-- name: UpdateFeedLastFetched :exec
UPDATE feeds SET last_fetched_at = ?, last_error = ?, updated_at = datetime('now') WHERE id = ?;

-- name: UpdateFeed :exec
UPDATE feeds SET name = ?, url = ?, feed_type = ?, scraper_module = ?, scraper_config = ?, fetch_interval_minutes = ?, updated_at = datetime('now') WHERE id = ?;

-- name: DeleteFeed :exec
DELETE FROM feeds WHERE id = ?;

-- name: GetFeedUnreadCount :one
SELECT COUNT(*) as count FROM articles WHERE feed_id = ? AND is_read = 0;

-- name: CreateCategory :one
INSERT INTO categories (name) VALUES (?) RETURNING *;

-- name: ListCategories :many
SELECT * FROM categories ORDER BY name;

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = ?;

-- name: AddFeedToCategory :exec
INSERT OR IGNORE INTO feed_categories (feed_id, category_id) VALUES (?, ?);

-- name: RemoveFeedFromCategory :exec
DELETE FROM feed_categories WHERE feed_id = ? AND category_id = ?;

-- name: GetFeedCategories :many
SELECT c.* FROM categories c
JOIN feed_categories fc ON c.id = fc.category_id
WHERE fc.feed_id = ?;
