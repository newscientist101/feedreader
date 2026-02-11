-- name: CreateScraperModule :one
INSERT INTO scraper_modules (name, description, script, script_type)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetScraperModule :one
SELECT * FROM scraper_modules WHERE id = ?;

-- name: GetScraperModuleByName :one
SELECT * FROM scraper_modules WHERE name = ?;

-- name: ListScraperModules :many
SELECT * FROM scraper_modules ORDER BY name;

-- name: UpdateScraperModule :exec
UPDATE scraper_modules SET name = ?, description = ?, script = ?, script_type = ?, updated_at = datetime('now') WHERE id = ?;

-- name: DeleteScraperModule :exec
DELETE FROM scraper_modules WHERE id = ?;

-- name: EnableScraperModule :exec
UPDATE scraper_modules SET enabled = 1 WHERE id = ?;

-- name: DisableScraperModule :exec
UPDATE scraper_modules SET enabled = 0 WHERE id = ?;
