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
    role         INT          NOT NULL DEFAULT 1,
    status       INT          NOT NULL DEFAULT 1,
    group_name   VARCHAR(64)  NOT NULL DEFAULT 'default',
    kyc_status   VARCHAR(32)  NOT NULL DEFAULT 'none',
    remark       TEXT         NOT NULL DEFAULT '',
    tenant_id    BIGINT       NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS users;
