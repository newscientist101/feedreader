-- Folder/Category settings and exclusion rules
CREATE TABLE category_settings (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  setting_key TEXT NOT NULL,
  setting_value TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(category_id, setting_key)
);

-- Exclusion rules for categories
CREATE TABLE category_exclusions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  exclusion_type TEXT NOT NULL, -- 'author', 'keyword'
  pattern TEXT NOT NULL,
  is_regex INTEGER DEFAULT 0,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_category_exclusions_category ON category_exclusions(category_id);

INSERT INTO migrations (migration_number, migration_name) VALUES (2, '002-folder-settings.sql');
