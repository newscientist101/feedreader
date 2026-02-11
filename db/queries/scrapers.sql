-- name: CreateScraperModule :one
INSERT INTO scraper_modules (name, description, script, script_type, user_id)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetScraperModule :one
SELECT * FROM scraper_modules WHERE id = ? AND user_id = ?;

-- name: GetScraperModuleByName :one
SELECT * FROM scraper_modules WHERE name = ? AND user_id = ?;

-- name: ListScraperModules :many
SELECT * FROM scraper_modules WHERE user_id = ? ORDER BY name;

-- name: UpdateScraperModule :exec
UPDATE scraper_modules SET name = ?, description = ?, script = ?, script_type = ?, updated_at = datetime('now') WHERE id = ? AND user_id = ?;

-- name: DeleteScraperModule :exec
DELETE FROM scraper_modules WHERE id = ? AND user_id = ?;

-- name: EnableScraperModule :exec
UPDATE scraper_modules SET enabled = 1 WHERE id = ? AND user_id = ?;

-- name: DisableScraperModule :exec
UPDATE scraper_modules SET enabled = 0 WHERE id = ? AND user_id = ?;

-- name: GetScraperModuleInternal :one
-- Internal use only - gets scraper by name for background fetching
SELECT * FROM scraper_modules WHERE name = ?;
