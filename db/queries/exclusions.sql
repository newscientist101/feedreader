-- name: CreateExclusion :one
INSERT INTO category_exclusions (category_id, exclusion_type, pattern, is_regex)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetExclusion :one
SELECT * FROM category_exclusions WHERE id = ?;

-- name: ListExclusionsByCategory :many
SELECT * FROM category_exclusions WHERE category_id = ? ORDER BY exclusion_type, pattern;

-- name: ListAllExclusions :many
SELECT e.*, c.name as category_name 
FROM category_exclusions e
JOIN categories c ON e.category_id = c.id
ORDER BY c.name, e.exclusion_type, e.pattern;

-- name: DeleteExclusion :exec
DELETE FROM category_exclusions WHERE id = ?;

-- name: DeleteExclusionsByCategory :exec
DELETE FROM category_exclusions WHERE category_id = ?;

-- name: UpdateExclusion :exec
UPDATE category_exclusions SET pattern = ?, is_regex = ? WHERE id = ?;

-- name: GetCategorySetting :one
SELECT * FROM category_settings WHERE category_id = ? AND setting_key = ?;

-- name: SetCategorySetting :exec
INSERT INTO category_settings (category_id, setting_key, setting_value)
VALUES (?, ?, ?)
ON CONFLICT(category_id, setting_key) DO UPDATE SET setting_value = excluded.setting_value;

-- name: ListCategorySettings :many
SELECT * FROM category_settings WHERE category_id = ?;

-- name: DeleteCategorySetting :exec
DELETE FROM category_settings WHERE category_id = ? AND setting_key = ?;
