-- +goose Up
CREATE TABLE provider_applications (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL DEFAULT 0,
    company       VARCHAR(256) NOT NULL DEFAULT '',
    contact       VARCHAR(128) NOT NULL DEFAULT '',
    email         VARCHAR(256) NOT NULL DEFAULT '',
    website       VARCHAR(512) NOT NULL DEFAULT '',
    bio           TEXT NOT NULL DEFAULT '',
    model_name    VARCHAR(256) NOT NULL DEFAULT '',
    model_family  VARCHAR(128) NOT NULL DEFAULT '',
    api_endpoint  VARCHAR(512) NOT NULL DEFAULT '',
    documentation VARCHAR(512) NOT NULL DEFAULT '',
    status        VARCHAR(16) NOT NULL DEFAULT 'pending',
    review_note   TEXT NOT NULL DEFAULT '',
    reviewed_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_provider_applications_status ON provider_applications(status, created_at);

-- +goose Down
DROP TABLE IF EXISTS provider_applications;
