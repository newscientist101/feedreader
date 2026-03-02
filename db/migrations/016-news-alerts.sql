-- News alerts: user-defined keyword/regex patterns that flag matching articles

CREATE TABLE news_alerts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  pattern TEXT NOT NULL,
  is_regex INTEGER NOT NULL DEFAULT 0,
  match_field TEXT NOT NULL DEFAULT 'title_summary',
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_news_alerts_user ON news_alerts(user_id);

CREATE TABLE article_alerts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
  alert_id INTEGER NOT NULL REFERENCES news_alerts(id) ON DELETE CASCADE,
  matched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  dismissed INTEGER NOT NULL DEFAULT 0,
  UNIQUE(article_id, alert_id)
);

CREATE INDEX idx_article_alerts_alert ON article_alerts(alert_id);
CREATE INDEX idx_article_alerts_article ON article_alerts(article_id);
