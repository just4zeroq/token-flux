-- +goose Up
CREATE TABLE IF NOT EXISTS llm_channel_models (
    id                  BIGSERIAL PRIMARY KEY,
    channel_id          BIGINT NOT NULL DEFAULT 0,
    model_spec_id       BIGINT NOT NULL DEFAULT 0,
    upstream_model_name VARCHAR(255) NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_channel_models_channel ON llm_channel_models(channel_id);
CREATE INDEX IF NOT EXISTS idx_channel_models_model ON llm_channel_models(model_spec_id);

-- +goose Down
DROP TABLE IF EXISTS llm_channel_models;
