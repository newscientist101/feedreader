-- name: CreateExclusion :one
INSERT INTO category_exclusions (category_id, exclusion_type, pattern, is_regex)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetExclusion :one
SELECT e.* FROM category_exclusions e
JOIN categories c ON e.category_id = c.id
WHERE e.id = ? AND c.user_id = ?;

-- name: ListExclusionsByCategory :many
SELECT e.* FROM category_exclusions e
JOIN categories c ON e.category_id = c.id
WHERE e.category_id = ? AND c.user_id = ?
ORDER BY e.exclusion_type, e.pattern;

-- name: ListAllExclusions :many
SELECT e.*, c.name as category_name 
FROM category_exclusions e
JOIN categories c ON e.category_id = c.id
WHERE c.user_id = ?
ORDER BY c.name, e.exclusion_type, e.pattern;

-- name: DeleteExclusion :exec
DELETE FROM category_exclusions 
WHERE category_exclusions.id = ? AND category_id IN (SELECT categories.id FROM categories WHERE categories.user_id = ?);

-- name: DeleteExclusionsByCategory :exec
DELETE FROM category_exclusions WHERE category_id = ?;

-- name: UpdateExclusion :exec
UPDATE category_exclusions SET pattern = ?, is_regex = ? 
WHERE category_exclusions.id = ? AND category_id IN (SELECT categories.id FROM categories WHERE categories.user_id = ?);

-- name: GetCategorySetting :one
SELECT cs.* FROM category_settings cs
JOIN categories c ON cs.category_id = c.id
WHERE cs.category_id = ? AND cs.setting_key = ? AND c.user_id = ?;

-- name: SetCategorySetting :exec
INSERT INTO category_settings (category_id, setting_key, setting_value)
VALUES (?, ?, ?)
ON CONFLICT(category_id, setting_key) DO UPDATE SET setting_value = excluded.setting_value;

-- name: ListCategorySettings :many
SELECT cs.* FROM category_settings cs
JOIN categories c ON cs.category_id = c.id
WHERE cs.category_id = ? AND c.user_id = ?;

-- name: DeleteCategorySetting :exec
DELETE FROM category_settings 
WHERE category_settings.category_id = ? AND setting_key = ? 
  AND category_settings.category_id IN (SELECT categories.id FROM categories WHERE categories.user_id = ?);
