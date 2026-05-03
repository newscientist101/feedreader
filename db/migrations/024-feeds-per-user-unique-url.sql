-- Replace the global UNIQUE(url) constraint on feeds with UNIQUE(url, user_id).
-- Two different users may subscribe to the same feed URL (e.g. the same Usenet
-- newsgroup); the constraint must only prevent a single user from duplicating
-- the same URL. Migration 005 documents that this change was applied manually
-- to the original production database; this migration performs it properly for
-- all environments including fresh installs and test databases.
--
-- SQLite does not support DROP INDEX ... or ALTER TABLE DROP CONSTRAINT, so we
-- use the recommended table-rebuild approach:
--   1. Create the new table with the correct constraints.
--   2. Copy all data.
--   3. Drop the old table.
--   4. Rename the new table.
--   5. Recreate dependent indexes and foreign-key-referencing tables.
--
-- Foreign keys that reference feeds(id) are CASCADE on DELETE and on UPDATE
-- (id never changes), so we only need to handle feed_categories which is the
-- one non-cascade-creating FK relationship rebuilt below.

PRAGMA foreign_keys = OFF;

CREATE TABLE feeds_new (
  id                   INTEGER   PRIMARY KEY AUTOINCREMENT,
  name                 TEXT      NOT NULL,
  url                  TEXT      NOT NULL,
  feed_type            TEXT      NOT NULL DEFAULT 'rss',
  scraper_module       TEXT,
  scraper_config       TEXT,
  last_fetched_at      TIMESTAMP,
  last_error           TEXT,
  fetch_interval_minutes INTEGER DEFAULT 60,
  created_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  user_id              INTEGER   REFERENCES users(id) ON DELETE CASCADE,
  content_filters      TEXT,
  site_url             TEXT      NOT NULL DEFAULT '',
  skip_retention       INTEGER   NOT NULL DEFAULT 0,
  consecutive_errors   INTEGER   NOT NULL DEFAULT 0,
  UNIQUE(url, user_id)
);

INSERT INTO feeds_new
  SELECT id, name, url, feed_type, scraper_module, scraper_config,
         last_fetched_at, last_error, fetch_interval_minutes, created_at,
         updated_at, user_id, content_filters, site_url, skip_retention,
         consecutive_errors
  FROM feeds;

DROP TABLE feeds;
ALTER TABLE feeds_new RENAME TO feeds;

-- Restore indexes that existed on the old table.
CREATE INDEX IF NOT EXISTS idx_feeds_user ON feeds(user_id);

PRAGMA foreign_keys = ON;

INSERT INTO migrations (migration_number, migration_name)
  VALUES (24, '024-feeds-per-user-unique-url.sql');
