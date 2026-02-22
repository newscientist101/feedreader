-- Add ON DELETE CASCADE to foreign keys referencing users(id)
-- SQLite doesn't support ALTER CONSTRAINT, so we recreate the tables.

-- 1. user_settings
CREATE TABLE user_settings_new (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  UNIQUE(user_id, key)
);
INSERT INTO user_settings_new SELECT user_id, key, value FROM user_settings;
DROP TABLE user_settings;
ALTER TABLE user_settings_new RENAME TO user_settings;
CREATE INDEX idx_user_settings_user ON user_settings(user_id);

-- 2. scraper_modules
CREATE TABLE scraper_modules_new (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  description TEXT,
  script TEXT NOT NULL,
  script_type TEXT NOT NULL DEFAULT 'javascript',
  enabled INTEGER DEFAULT 1,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  user_id INTEGER REFERENCES users(id) ON DELETE CASCADE
);
INSERT INTO scraper_modules_new SELECT id, name, description, script, script_type, enabled, created_at, updated_at, user_id FROM scraper_modules;
DROP TABLE scraper_modules;
ALTER TABLE scraper_modules_new RENAME TO scraper_modules;
CREATE INDEX idx_scraper_modules_user ON scraper_modules(user_id);

-- 3. category_settings
CREATE TABLE category_settings_new (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  setting_key TEXT NOT NULL,
  setting_value TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
  UNIQUE(category_id, setting_key)
);
INSERT INTO category_settings_new SELECT id, category_id, setting_key, setting_value, created_at, user_id FROM category_settings;
DROP TABLE category_settings;
ALTER TABLE category_settings_new RENAME TO category_settings;
