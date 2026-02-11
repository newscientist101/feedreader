-- Migrations table
CREATE TABLE IF NOT EXISTS migrations (
  migration_number INTEGER PRIMARY KEY,
  migration_name TEXT NOT NULL,
  executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO migrations (migration_number, migration_name) VALUES (1, '001-init.sql');

-- Feeds table
CREATE TABLE feeds (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  url TEXT NOT NULL UNIQUE,
  feed_type TEXT NOT NULL DEFAULT 'rss', -- 'rss', 'atom', 'scraper'
  scraper_module TEXT, -- name of scraper module for non-RSS feeds
  scraper_config TEXT, -- JSON config for scraper
  last_fetched_at TIMESTAMP,
  last_error TEXT,
  fetch_interval_minutes INTEGER DEFAULT 60,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Categories for organizing feeds
CREATE TABLE categories (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Feed-category mapping
CREATE TABLE feed_categories (
  feed_id INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
  PRIMARY KEY (feed_id, category_id)
);

-- Articles/items from feeds
CREATE TABLE articles (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  feed_id INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  guid TEXT NOT NULL, -- unique identifier from feed
  title TEXT NOT NULL,
  url TEXT,
  author TEXT,
  content TEXT,
  summary TEXT,
  image_url TEXT,
  published_at TIMESTAMP,
  fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  is_read INTEGER DEFAULT 0,
  is_starred INTEGER DEFAULT 0,
  UNIQUE(feed_id, guid)
);

CREATE INDEX idx_articles_feed_id ON articles(feed_id);
CREATE INDEX idx_articles_published_at ON articles(published_at DESC);
CREATE INDEX idx_articles_is_read ON articles(is_read);
CREATE INDEX idx_articles_is_starred ON articles(is_starred);

-- Scraper modules registry
CREATE TABLE scraper_modules (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  description TEXT,
  script TEXT NOT NULL, -- JavaScript/Lua script content
  script_type TEXT NOT NULL DEFAULT 'javascript', -- 'javascript' or 'lua'
  enabled INTEGER DEFAULT 1,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert default category
INSERT INTO categories (name) VALUES ('Uncategorized');
