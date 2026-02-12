-- Add content_filters column to feeds for per-feed content cleaning
ALTER TABLE feeds ADD COLUMN content_filters TEXT;
-- JSON array of patterns to remove from article content, e.g.:
-- [{"pattern": "<div class=\"ref-ar\">.*?</div>", "is_regex": true}]
