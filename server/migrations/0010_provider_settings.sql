-- +goose Up
-- +goose StatementBegin
CREATE TABLE provider_settings (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL DEFAULT 0,
    share_bps INT NOT NULL DEFAULT 7000,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_provider_settings_user_id ON provider_settings(user_id);
COMMENT ON TABLE provider_settings IS 'Provider configuration: default revenue share percentage';
COMMENT ON COLUMN provider_settings.share_bps IS 'Default revenue share in basis points (7000 = 70%)';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS provider_settings;
-- +goose StatementEnd
