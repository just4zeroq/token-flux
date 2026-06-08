-- +goose Up
CREATE TABLE IF NOT EXISTS nodes (
    id             BIGSERIAL PRIMARY KEY,
    wallet_address TEXT NOT NULL UNIQUE,
    name           TEXT NOT NULL DEFAULT '',
    signature      TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'active',
    last_seen_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    registered_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_nodes_wallet ON nodes(wallet_address);
CREATE INDEX IF NOT EXISTS idx_nodes_status ON nodes(status);

-- +goose Down
DROP TABLE IF EXISTS nodes;
