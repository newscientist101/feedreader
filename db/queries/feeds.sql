-- name: CreateFeed :one
INSERT INTO feeds (name, url, feed_type, scraper_module, scraper_config, fetch_interval_minutes, user_id)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetFeed :one
SELECT * FROM feeds WHERE id = ? AND user_id = ?;

-- name: GetFeedByURL :one
SELECT * FROM feeds WHERE url = ? AND user_id = ?;

-- name: ListFeeds :many
SELECT * FROM feeds WHERE user_id = ? ORDER BY name;

-- name: ListFeedsToFetch :many
SELECT * FROM feeds 
WHERE feed_type != 'newsletter'
  AND (last_fetched_at IS NULL 
   OR datetime(last_fetched_at, '+' || fetch_interval_minutes || ' minutes') < datetime('now'))
ORDER BY last_fetched_at NULLS FIRST;

-- name: UpdateFeedLastFetched :exec
UPDATE feeds SET last_fetched_at = ?, last_error = ?, updated_at = datetime('now') WHERE id = ?;

-- name: UpdateFeed :exec
UPDATE feeds SET name = ?, url = ?, feed_type = ?, scraper_module = ?, scraper_config = ?, fetch_interval_minutes = ?, content_filters = ?, updated_at = datetime('now') WHERE id = ? AND user_id = ?;

-- name: DeleteFeed :exec
DELETE FROM feeds WHERE id = ? AND user_id = ?;

-- name: GetFeedUnreadCount :one
SELECT COUNT(*) as count FROM articles WHERE feed_id = ? AND is_read = 0;

-- name: CreateCategory :one
INSERT INTO categories (name, user_id) VALUES (?, ?) RETURNING *;

-- name: ListCategories :many
SELECT * FROM categories WHERE user_id = ? ORDER BY sort_order, name;

-- name: UpdateCategorySortOrder :exec
UPDATE categories SET sort_order = ? WHERE id = ? AND user_id = ?;

-- name: UpdateCategoryParent :exec
UPDATE categories SET parent_id = ?, sort_order = ? WHERE id = ? AND user_id = ?;

-- name: GetChildCategories :many
SELECT * FROM categories WHERE parent_id = ? AND user_id = ? ORDER BY sort_order, name;

-- name: GetCategory :one
SELECT * FROM categories WHERE id = ? AND user_id = ?;

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = ? AND user_id = ?;

-- name: AddFeedToCategory :exec
INSERT OR IGNORE INTO feed_categories (feed_id, category_id) VALUES (?, ?);

-- name: RemoveFeedFromCategory :exec
DELETE FROM feed_categories WHERE feed_id = ? AND category_id = ?;

-- name: GetFeedCategories :many
SELECT c.* FROM categories c
JOIN feed_categories fc ON c.id = fc.category_id
WHERE fc.feed_id = ?;

-- name: GetCategoryByName :one
SELECT * FROM categories WHERE name = ? AND user_id = ?;

-- name: ListFeedsByCategory :many
SELECT f.* FROM feeds f
JOIN feed_categories fc ON f.id = fc.feed_id
WHERE fc.category_id = ? AND f.user_id = ?
ORDER BY f.name;

-- name: ListUncategorizedFeeds :many
SELECT f.* FROM feeds f
WHERE f.user_id = ? AND NOT EXISTS (
  SELECT 1 FROM feed_categories fc WHERE fc.feed_id = f.id
)
ORDER BY f.name;

-- name: GetCategoryUnreadCount :one
SELECT COUNT(*) as count FROM articles a
JOIN feeds f ON a.feed_id = f.id
JOIN feed_categories fc ON f.id = fc.feed_id
WHERE fc.category_id = ? AND a.is_read = 0;

-- name: UpdateCategory :exec
UPDATE categories SET name = ? WHERE id = ? AND user_id = ?;

-- name: ListFeedCategoryMappings :many
SELECT fc.feed_id, fc.category_id FROM feed_categories fc
JOIN feeds f ON fc.feed_id = f.id
WHERE f.user_id = ?;

-- name: ClearFeedCategories :exec
DELETE FROM feed_categories WHERE feed_id = ?;

-- name: GetFeedOwner :one
SELECT user_id FROM feeds WHERE id = ?;

-- name: UpdateFeedSiteURL :exec
UPDATE feeds SET site_url = ? WHERE id = ?;

-- name: GetAllFeedUnreadCounts :many
SELECT feed_id, COUNT(*) as count FROM articles
WHERE is_read = 0
  AND feed_id IN (SELECT id FROM feeds WHERE user_id = ?)
GROUP BY feed_id;

-- name: GetAllCategoryUnreadCounts :many
SELECT fc.category_id, COUNT(*) as count FROM articles a
JOIN feeds f ON a.feed_id = f.id
JOIN feed_categories fc ON f.id = fc.feed_id
WHERE a.is_read = 0 AND f.user_id = ?
GROUP BY fc.category_id;

-- name: UpdateFeedScraperConfig :exec
UPDATE feeds SET scraper_config = ?, updated_at = datetime('now') WHERE id = ?;
