-- +goose Up
-- Double-entry accounting core: accounts + transactions + transaction_entries.
-- All amounts in micro-units (BIGINT, value x 10^6) to avoid float drift.

CREATE TABLE accounts (
    id            BIGSERIAL PRIMARY KEY,
    owner_type    VARCHAR(32) NOT NULL,    -- user | platform
    owner_id      BIGINT      NOT NULL,
    asset         VARCHAR(32) NOT NULL,    -- credits | balance | points
    balance_micro BIGINT      NOT NULL DEFAULT 0,
    version       BIGINT      NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (owner_type, owner_id, asset)
);

CREATE INDEX idx_accounts_owner ON accounts(owner_type, owner_id);

CREATE TABLE transactions (
    id         BIGSERIAL PRIMARY KEY,
    tx_type    VARCHAR(32) NOT NULL,
    -- deposit_credits | exchange | consume | settle | withdraw | points_grant
    ref_type   VARCHAR(64),
    -- chain_deposit | order | usage_record | withdraw_request
    ref_id     BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transactions_type ON transactions(tx_type);
CREATE INDEX idx_transactions_ref ON transactions(ref_type, ref_id);

CREATE TABLE transaction_entries (
    id                  BIGSERIAL PRIMARY KEY,
    tx_id               BIGINT NOT NULL REFERENCES transactions(id),
    account_id          BIGINT NOT NULL REFERENCES accounts(id),
    delta_micro         BIGINT NOT NULL,
    balance_after_micro BIGINT NOT NULL
);

CREATE INDEX idx_transaction_entries_tx ON transaction_entries(tx_id);
CREATE INDEX idx_transaction_entries_account ON transaction_entries(account_id);

-- +goose Down
DROP TABLE IF EXISTS transaction_entries;
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS accounts;
