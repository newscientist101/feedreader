-- Add site_url column to feeds for the website URL (distinct from the feed URL)
ALTER TABLE feeds ADD COLUMN site_url TEXT NOT NULL DEFAULT '';
