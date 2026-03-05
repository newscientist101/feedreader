-- Add skip_retention flag to feeds
ALTER TABLE feeds ADD COLUMN skip_retention INTEGER NOT NULL DEFAULT 0;
