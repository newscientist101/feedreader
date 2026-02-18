CREATE TABLE IF NOT EXISTS history_articles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    viewed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_history_articles_user_id ON history_articles(user_id);
CREATE INDEX IF NOT EXISTS idx_history_articles_viewed_at ON history_articles(viewed_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_history_articles_user_article ON history_articles(user_id, article_id);

INSERT INTO migrations (migration_number, migration_name) VALUES (14, '014-history.sql');
