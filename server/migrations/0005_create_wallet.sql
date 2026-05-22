-- +goose Up
CREATE TABLE deposit_addresses (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT       NOT NULL REFERENCES users(id),
    chain      VARCHAR(32)  NOT NULL,
    address    VARCHAR(255) NOT NULL,
    hd_path    VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, chain),
    UNIQUE (chain, address)
);

CREATE TABLE chain_deposits (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT          NOT NULL REFERENCES users(id),
    chain              VARCHAR(32)     NOT NULL,
    tx_hash            VARCHAR(128)    NOT NULL,
    from_addr          VARCHAR(255)    NOT NULL,
    to_addr            VARCHAR(255)    NOT NULL,
    amount_native      NUMERIC(38, 18) NOT NULL,
    amount_usd_at_time BIGINT          NOT NULL,
    rate_at_time       NUMERIC(38, 18) NOT NULL,
    confirmations      INT             NOT NULL DEFAULT 0,
    status             VARCHAR(32)     NOT NULL DEFAULT 'observed',
    observed_at        TIMESTAMPTZ     NOT NULL DEFAULT NOW(),
    UNIQUE (chain, tx_hash)
);

CREATE INDEX idx_chain_deposits_user ON chain_deposits(user_id);
CREATE INDEX idx_chain_deposits_status ON chain_deposits(status);

CREATE TABLE withdraw_requests (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT       NOT NULL REFERENCES users(id),
    chain           VARCHAR(32)  NOT NULL,
    to_address      VARCHAR(255) NOT NULL,
    amount_balance  BIGINT       NOT NULL,
    fee             BIGINT       NOT NULL DEFAULT 0,
    status          VARCHAR(32)  NOT NULL DEFAULT 'pending',
    tx_hash         VARCHAR(128),
    broadcast_at    TIMESTAMPTZ,
    confirmed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_withdraw_requests_user ON withdraw_requests(user_id);
CREATE INDEX idx_withdraw_requests_status ON withdraw_requests(status);

-- +goose Down
DROP TABLE IF EXISTS withdraw_requests;
DROP TABLE IF EXISTS chain_deposits;
DROP TABLE IF EXISTS deposit_addresses;
