CREATE TABLE IF NOT EXISTS queue_articles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    added_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, article_id)
);

CREATE INDEX IF NOT EXISTS idx_queue_articles_user_id ON queue_articles(user_id);
CREATE INDEX IF NOT EXISTS idx_queue_articles_added_at ON queue_articles(added_at ASC);

INSERT INTO migrations (migration_number, migration_name) VALUES (12, '012-queue.sql');
