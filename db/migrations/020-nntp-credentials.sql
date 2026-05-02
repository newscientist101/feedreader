-- Per-user Eternal September NNTP credentials.
-- The password is stored as AES-GCM ciphertext; the nonce is prepended to the
-- ciphertext bytes and the whole blob is stored as hex in password_enc.
-- The encryption key comes from server configuration (NNTP_KEY env variable).
CREATE TABLE IF NOT EXISTS nntp_credentials (
  user_id     INTEGER NOT NULL PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  username    TEXT    NOT NULL,
  password_enc TEXT   NOT NULL,  -- hex-encoded nonce||ciphertext from AES-GCM
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO migrations (migration_number, migration_name)
  VALUES (20, '020-nntp-credentials.sql');
