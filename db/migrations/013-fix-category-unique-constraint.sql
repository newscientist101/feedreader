-- pragma:disable_fk
-- Fix categories unique constraint: replace global UNIQUE(name) with per-user UNIQUE(name, user_id).
-- Migration 005 was a no-op; this performs the actual schema change.
-- SQLite doesn't support DROP CONSTRAINT, so we recreate the table.

CREATE TABLE categories_new (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
  sort_order INTEGER DEFAULT 0,
  parent_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
  UNIQUE(name, user_id)
);

INSERT INTO categories_new (id, name, created_at, user_id, sort_order, parent_id)
  SELECT id, name, created_at, user_id, sort_order, parent_id FROM categories;

DROP TABLE categories;
ALTER TABLE categories_new RENAME TO categories;

CREATE INDEX idx_categories_user ON categories(user_id);
CREATE INDEX idx_categories_parent_id ON categories(parent_id);
