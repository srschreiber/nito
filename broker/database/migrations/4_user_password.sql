CREATE EXTENSION IF NOT EXISTS "pgcrypto";

ALTER TABLE users ADD COLUMN password_hash VARCHAR NOT NULL;
-- example
-- UPDATE users SET password_hash = crypt('plaintext_password', gen_salt('bf')) WHERE id = 'some-user-id';
-- bf for blowfish