-- usenet_article_meta: companion table for NNTP articles that stores
-- Usenet-specific identity and threading metadata.
-- Each row is keyed by articles.id (one-to-one) and also carries a
-- denormalised feed_id FK for efficient group-scoped lookups.
--
-- message_id          : the canonical Message-ID header value (preserved as-is)
-- references_header   : the raw References header value (preserved as-is, nullable)
-- parent_message_id   : Message-ID of the direct parent (last References entry, nullable)
-- root_message_id     : Message-ID of the thread root (first References entry, or own ID)
-- group_name          : canonical lowercase newsgroup name (e.g. comp.lang.go)
-- article_number      : numeric article number within the group
CREATE TABLE IF NOT EXISTS usenet_article_meta (
  article_id        INTEGER NOT NULL PRIMARY KEY REFERENCES articles(id) ON DELETE CASCADE,
  feed_id           INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  message_id        TEXT    NOT NULL,
  references_header TEXT,
  parent_message_id TEXT,
  root_message_id   TEXT    NOT NULL,
  group_name        TEXT    NOT NULL,
  article_number    INTEGER NOT NULL,
  created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(feed_id, article_number),
  UNIQUE(feed_id, message_id)
);

CREATE INDEX IF NOT EXISTS idx_usenet_meta_root_message_id
  ON usenet_article_meta(root_message_id);

CREATE INDEX IF NOT EXISTS idx_usenet_meta_parent_message_id
  ON usenet_article_meta(parent_message_id);

INSERT INTO migrations (migration_number, migration_name)
  VALUES (23, '023-usenet-article-meta.sql');
