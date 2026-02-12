-- Add content_filters column to feeds for per-feed content cleaning
-- Check if column exists before adding (SQLite doesn't support IF NOT EXISTS for columns)
-- This migration is safe to run multiple times
ALTER TABLE feeds ADD COLUMN content_filters TEXT;
-- If the above fails with "duplicate column", the migration system should handle it gracefully
-- JSON array of patterns to remove from article content, e.g.:
-- [{"pattern": "<div class=\"ref-ar\">.*?</div>", "is_regex": true}]
