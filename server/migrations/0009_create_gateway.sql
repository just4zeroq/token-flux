-- +goose Up
-- gateway-internal: upstream channel routing for catalog_items of type=llm.
-- One catalog_item may map to multiple upstream channels for failover/priority.
CREATE TABLE upstream_channels (
    id            BIGSERIAL PRIMARY KEY,
    item_id       BIGINT       NOT NULL REFERENCES catalog_items(id),
    provider      VARCHAR(64)  NOT NULL,    -- openai | anthropic | azure | ...
    base_url      VARCHAR(512) NOT NULL,
    key_encrypted TEXT         NOT NULL,
    priority      INT          NOT NULL DEFAULT 0,
    status        VARCHAR(32)  NOT NULL DEFAULT 'active',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_upstream_channels_item ON upstream_channels(item_id);
CREATE INDEX idx_upstream_channels_status ON upstream_channels(status);

-- +goose Down
DROP TABLE IF EXISTS upstream_channels;
