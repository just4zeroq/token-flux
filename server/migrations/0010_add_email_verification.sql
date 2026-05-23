-- +goose Up
UPDATE users SET email = NULL WHERE email = '';
ALTER TABLE users ADD CONSTRAINT users_email_unique UNIQUE (email);

CREATE TABLE email_verifications (
    id         BIGSERIAL PRIMARY KEY,
    email      VARCHAR(255) NOT NULL,
    code       VARCHAR(16)  NOT NULL,
    purpose    VARCHAR(32)  NOT NULL DEFAULT 'register',
    expires_at TIMESTAMPTZ  NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_email_verifications_email_purpose ON email_verifications(email, purpose, used_at, expires_at);

-- +goose Down
DROP TABLE IF EXISTS email_verifications;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_unique;
