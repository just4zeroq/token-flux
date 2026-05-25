-- +goose Up
-- Seed the `platform` system user. LLM management resources (channels, models,
-- prices, etc.) are attributed to this user so admin-owned records have a real
-- foreign-key target. The password is an unusable sentinel — this account is
-- not intended to be logged into.
INSERT INTO users (username, password, email, role, status)
VALUES ('platform', '!disabled!', 'platform@local', 100, 1)
ON CONFLICT (username) DO NOTHING;

-- +goose Down
DELETE FROM users WHERE username = 'platform';
