-- +goose Up
CREATE TABLE usage_records (
    id                       BIGSERIAL PRIMARY KEY,
    user_id                  BIGINT       NOT NULL REFERENCES users(id),
    api_key_id               BIGINT       REFERENCES api_keys(id),
    item_id                  BIGINT       NOT NULL REFERENCES catalog_items(id),
    runtime                  VARCHAR(32)  NOT NULL,            -- llm | mcp | agent
    capability_kind          VARCHAR(64)  NOT NULL DEFAULT '', -- chat | embedding | mcp_call | agent_exec
    input_tokens             INT          NOT NULL DEFAULT 0,
    output_tokens            INT          NOT NULL DEFAULT 0,
    call_count               INT          NOT NULL DEFAULT 1,
    latency_ms               INT          NOT NULL DEFAULT 0,
    cost_credits             BIGINT       NOT NULL DEFAULT 0,
    commission_credits       BIGINT       NOT NULL DEFAULT 0,
    provider_revenue_credits BIGINT       NOT NULL DEFAULT 0,
    points_to_consumer       BIGINT       NOT NULL DEFAULT 0,
    points_to_provider       BIGINT       NOT NULL DEFAULT 0,
    request_id               VARCHAR(128) NOT NULL DEFAULT '',
    channel_id               BIGINT,
    ip                       VARCHAR(64)  NOT NULL DEFAULT '',
    is_stream                BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_usage_records_user_created ON usage_records(user_id, created_at);
CREATE INDEX idx_usage_records_item_created ON usage_records(item_id, created_at);
CREATE INDEX idx_usage_records_request_id ON usage_records(request_id);

-- +goose Down
DROP TABLE IF EXISTS usage_records;
