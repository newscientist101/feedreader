-- User settings table for per-user key-value preferences
CREATE TABLE IF NOT EXISTS user_settings (
  user_id INTEGER NOT NULL REFERENCES users(id),
  key TEXT NOT NULL,
  value TEXT NOT NULL,
  UNIQUE(user_id, key)
);

CREATE INDEX idx_user_settings_user ON user_settings(user_id);
