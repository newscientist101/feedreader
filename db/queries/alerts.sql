-- name: CreateAlert :one
INSERT INTO news_alerts (user_id, name, pattern, is_regex, match_field)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetAlert :one
SELECT * FROM news_alerts WHERE id = ? AND user_id = ?;

-- name: ListAlertsByUser :many
SELECT * FROM news_alerts WHERE user_id = ? ORDER BY created_at DESC;

-- name: UpdateAlert :one
UPDATE news_alerts
SET name = ?, pattern = ?, is_regex = ?, match_field = ?
WHERE id = ? AND user_id = ?
RETURNING *;

-- name: DeleteAlert :exec
DELETE FROM news_alerts WHERE id = ? AND user_id = ?;

-- name: InsertArticleAlert :exec
INSERT OR IGNORE INTO article_alerts (article_id, alert_id) VALUES (?, ?);

-- name: ListAlertArticlesGrouped :many
SELECT aa.id, aa.article_id, aa.alert_id, aa.matched_at, aa.dismissed,
       a.title as article_title, a.url as article_url, a.summary as article_summary,
       a.published_at as article_published_at,
       f.name as feed_name,
       na.name as alert_name
FROM article_alerts aa
JOIN articles a ON aa.article_id = a.id
JOIN feeds f ON a.feed_id = f.id
JOIN news_alerts na ON aa.alert_id = na.id
WHERE na.user_id = ? AND aa.dismissed = 0
ORDER BY na.id, aa.matched_at DESC
LIMIT ? OFFSET ?;

-- name: ListAlertArticles :many
SELECT aa.id, aa.article_id, aa.alert_id, aa.matched_at, aa.dismissed,
       a.title as article_title, a.url as article_url, a.summary as article_summary,
       a.published_at as article_published_at,
       f.name as feed_name
FROM article_alerts aa
JOIN articles a ON aa.article_id = a.id
JOIN feeds f ON a.feed_id = f.id
JOIN news_alerts na ON aa.alert_id = na.id
WHERE aa.alert_id = ? AND na.user_id = ?
ORDER BY aa.matched_at DESC
LIMIT ? OFFSET ?;

-- name: CountUndismissedAlerts :one
SELECT COUNT(*) FROM article_alerts aa
JOIN news_alerts na ON aa.alert_id = na.id
WHERE na.user_id = ? AND aa.dismissed = 0;

-- name: DismissArticleAlert :exec
UPDATE article_alerts SET dismissed = 1
WHERE article_alerts.id = ? AND alert_id IN (SELECT news_alerts.id FROM news_alerts WHERE user_id = ?);

-- name: DismissAllForAlert :exec
UPDATE article_alerts SET dismissed = 1
WHERE alert_id = ? AND alert_id IN (SELECT id FROM news_alerts WHERE user_id = ?);

-- name: ListUnreadArticlesByFeedForAlerts :many
SELECT a.id, a.feed_id, a.title, a.url, a.author, a.content, a.summary,
       a.published_at, a.fetched_at
FROM articles a
JOIN feeds f ON a.feed_id = f.id
WHERE a.feed_id = ? AND f.user_id = ? AND a.is_read = 0
ORDER BY a.fetched_at DESC;
