-- Track GUIDs of articles that have been hard-deleted by retention cleanup.
-- Prevents re-insertion of old articles whose GUIDs were removed from the
-- articles table by the retention policy.
CREATE TABLE seen_guids (
  feed_id INTEGER NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  guid    TEXT NOT NULL,
  seen_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (feed_id, guid)
);

CREATE INDEX idx_seen_guids_seen_at ON seen_guids(seen_at);
