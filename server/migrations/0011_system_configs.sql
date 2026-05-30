-- +goose Up
-- +goose StatementBegin
CREATE TABLE system_configs (
    key         VARCHAR(128) PRIMARY KEY,
    value       TEXT NOT NULL DEFAULT '',
    description VARCHAR(512) NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE system_configs IS 'Key-value system configuration for tunable parameters';
COMMENT ON COLUMN system_configs.key IS 'Config key, e.g. gateway.max_retries, gateway.rate_limit_default_qps';

INSERT INTO system_configs (key, value, description) VALUES
    ('gateway.max_retries', '3', 'Maximum retry attempts for gateway upstream calls'),
    ('gateway.rate_limit_default_qps', '60', 'Default rate limit QPS per API key');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS system_configs;
-- +goose StatementEnd
