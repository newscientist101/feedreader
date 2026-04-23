-- Track consecutive fetch errors for backoff
ALTER TABLE feeds ADD COLUMN consecutive_errors INTEGER NOT NULL DEFAULT 0;
