-- usenet_feed_state: NNTP-specific companion table for feeds with feed_type='nntp'.
-- Each subscribed newsgroup has one row here keyed by feed_id.
-- The feeds table (with feed_type='nntp') remains the authoritative source for
-- display name, folder assignment, last_fetched_at, and error state.
--
-- high_water_article_number: the highest article number fetched so far.
-- 0 means no articles have been fetched yet (first fetch imports newest 100).
-- group_name: lowercase canonical newsgroup name (e.g. comp.lang.go)
-- provider: fixed to 'eternal-september' for now.
CREATE TABLE IF NOT EXISTS usenet_feed_state (
  feed_id                  INTEGER NOT NULL PRIMARY KEY REFERENCES feeds(id) ON DELETE CASCADE,
  provider                 TEXT    NOT NULL DEFAULT 'eternal-september',
  group_name               TEXT    NOT NULL,
  high_water_article_number INTEGER NOT NULL DEFAULT 0,
  created_at               TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at               TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  -- Prevent a user from subscribing to the same group twice via the same provider.
  -- The feed_id FK already references the feeds table which is user-scoped,
  -- so this unique constraint ensures uniqueness per provider+group within a user's feeds.
  UNIQUE(provider, group_name, feed_id)
);

CREATE INDEX IF NOT EXISTS idx_usenet_feed_state_group_name ON usenet_feed_state(group_name);

INSERT INTO migrations (migration_number, migration_name)
  VALUES (22, '022-usenet-feed-state.sql');
