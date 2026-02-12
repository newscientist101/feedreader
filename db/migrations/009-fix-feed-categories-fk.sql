-- Fix the feed_categories foreign key reference which was broken during migration 004
-- Migration 004's ALTER TABLE caused feeds table to be recreated as feeds_old,
-- breaking the foreign key in feed_categories

DROP TABLE IF EXISTS feed_categories;

-- Recreate feed_categories with correct foreign key reference to feeds
CREATE TABLE feed_categories (
  feed_id INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  PRIMARY KEY (feed_id, category_id)
);

CREATE INDEX IF NOT EXISTS idx_feed_categories_category_id ON feed_categories(category_id);

-- Re-insert migration record
INSERT INTO migrations (migration_number, migration_name) VALUES (9, '009-fix-feed-categories-fk.sql');
