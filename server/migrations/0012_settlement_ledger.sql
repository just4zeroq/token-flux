-- +goose Up
CREATE TABLE IF NOT EXISTS settlement_records (
    id BIGSERIAL PRIMARY KEY,
    product_type VARCHAR(32) NOT NULL,
    ref_type VARCHAR(64) NOT NULL,
    ref_id BIGINT NOT NULL,
    consumer_user_id BIGINT NOT NULL REFERENCES users(id),
    provider_user_id BIGINT REFERENCES users(id),
    cost_credits BIGINT NOT NULL DEFAULT 0,
    provider_revenue_credits BIGINT NOT NULL DEFAULT 0,
    commission_credits BIGINT NOT NULL DEFAULT 0,
    points_to_consumer BIGINT NOT NULL DEFAULT 0,
    points_to_provider BIGINT NOT NULL DEFAULT 0,
    transaction_id BIGINT REFERENCES transactions(id),
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at TIMESTAMPTZ NULL,
    UNIQUE(ref_type, ref_id)
);

CREATE TABLE IF NOT EXISTS ledger_snapshots (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id),
    asset VARCHAR(32) NOT NULL,
    balance_micro BIGINT NOT NULL,
    snapshot_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_settlement_records_consumer ON settlement_records(consumer_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_settlement_records_provider ON settlement_records(provider_user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_settlement_records_status ON settlement_records(status, created_at);
CREATE INDEX IF NOT EXISTS idx_ledger_snapshots_account_time ON ledger_snapshots(account_id, snapshot_at);

-- +goose Down
DROP TABLE IF EXISTS ledger_snapshots;
DROP TABLE IF EXISTS settlement_records;
