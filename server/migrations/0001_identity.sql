-- +goose Up
CREATE TABLE users (
    id           BIGSERIAL PRIMARY KEY,
    username     VARCHAR(255) NOT NULL UNIQUE,
    password     VARCHAR(255) NOT NULL,
    email        VARCHAR(255) NOT NULL DEFAULT '',
    phone        VARCHAR(64)  NOT NULL DEFAULT '',
    display_name VARCHAR(255) NOT NULL DEFAULT '',
    avatar       VARCHAR(512) NOT NULL DEFAULT '',
    source       VARCHAR(64)  NOT NULL DEFAULT 'email',
    role         INT          NOT NULL DEFAULT 0,
    status       INT          NOT NULL DEFAULT 1,
    group_name   VARCHAR(64)  NOT NULL DEFAULT 'default',
    kyc_status   VARCHAR(32)  NOT NULL DEFAULT 'none',
    remark       TEXT         NOT NULL DEFAULT '',
    tenant_id    BIGINT       NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX users_email_unique ON users(email) WHERE email != '';

CREATE TABLE api_keys (
    id                   BIGSERIAL PRIMARY KEY,
    user_id              BIGINT       NOT NULL,
    key                  VARCHAR(255) NOT NULL UNIQUE,
    name                 VARCHAR(255) NOT NULL DEFAULT '',
    status               INT          NOT NULL DEFAULT 1,
    disabled_reason      VARCHAR(64)  NOT NULL DEFAULT '',
    expire_time          TIMESTAMPTZ,
    model_limits         TEXT         NOT NULL DEFAULT '',
    model_limits_enabled BOOLEAN      NOT NULL DEFAULT FALSE,
    group_name           VARCHAR(64)  NOT NULL DEFAULT 'default',
    quota_credits        BIGINT,
    remain_quota         BIGINT       NOT NULL DEFAULT 0,
    used_quota           BIGINT       NOT NULL DEFAULT 0,
    unlimited_quota      BOOLEAN      NOT NULL DEFAULT FALSE,
    allow_ips            TEXT         NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ
);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_key ON api_keys(key);

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
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS users;
