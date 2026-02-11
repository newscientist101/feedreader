-- Migration applied manually - unique constraints are now per-user
-- feeds: UNIQUE(url, user_id)
-- categories: UNIQUE(name, user_id)
SELECT 1;
