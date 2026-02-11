-- Users table
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  external_id TEXT NOT NULL UNIQUE,  -- X-ExeDev-UserID
  email TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_seen_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Add user_id to existing tables
ALTER TABLE feeds ADD COLUMN user_id INTEGER REFERENCES users(id);
ALTER TABLE categories ADD COLUMN user_id INTEGER REFERENCES users(id);
ALTER TABLE scraper_modules ADD COLUMN user_id INTEGER REFERENCES users(id);
ALTER TABLE category_settings ADD COLUMN user_id INTEGER REFERENCES users(id);

-- Create indexes for user lookups
CREATE INDEX idx_feeds_user ON feeds(user_id);
CREATE INDEX idx_categories_user ON categories(user_id);
CREATE INDEX idx_scraper_modules_user ON scraper_modules(user_id);
