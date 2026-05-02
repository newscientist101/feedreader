-- Add key_version to nntp_credentials to track which encryption key version
-- was used to encrypt the password. This allows future USENET_CREDENTIAL_KEY
-- rotation to identify which rows need re-encryption. The initial version is 'v1'.
ALTER TABLE nntp_credentials ADD COLUMN key_version TEXT NOT NULL DEFAULT 'v1';

INSERT INTO migrations (migration_number, migration_name)
  VALUES (21, '021-nntp-key-version.sql');
