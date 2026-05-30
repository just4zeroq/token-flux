-- +goose Up
CREATE TABLE payment_channels (
    id          BIGSERIAL PRIMARY KEY,
    channel     VARCHAR(32) NOT NULL DEFAULT '',
    name        VARCHAR(128) NOT NULL DEFAULT '',
    app_id      VARCHAR(64) NOT NULL DEFAULT '',
    private_key TEXT NOT NULL DEFAULT '',
    public_key  TEXT NOT NULL DEFAULT '',
    api_key     VARCHAR(256) NOT NULL DEFAULT '',
    cert_sn     VARCHAR(64) NOT NULL DEFAULT '',
    notify_url  VARCHAR(512) NOT NULL DEFAULT '',
    is_prod     INT NOT NULL DEFAULT 0,
    status      INT NOT NULL DEFAULT 1,
    config_json TEXT NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE payment_orders (
    id              BIGSERIAL PRIMARY KEY,
    order_no        VARCHAR(64) NOT NULL,
    user_id         BIGINT NOT NULL DEFAULT 0,
    channel         VARCHAR(32) NOT NULL DEFAULT '',
    amount_credits  BIGINT NOT NULL DEFAULT 0,
    amount_fiat     DECIMAL(12,2) NOT NULL DEFAULT 0,
    currency        VARCHAR(8) NOT NULL DEFAULT 'CNY',
    trade_no        VARCHAR(128) NOT NULL DEFAULT '',
    status          VARCHAR(16) NOT NULL DEFAULT 'pending',
    notify_raw      TEXT NOT NULL DEFAULT '',
    paid_at         TIMESTAMPTZ,
    credited_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX idx_payment_orders_order_no ON payment_orders(order_no);
CREATE INDEX idx_payment_orders_user ON payment_orders(user_id, created_at DESC);
CREATE INDEX idx_payment_orders_status ON payment_orders(status, created_at);

-- +goose Down
DROP TABLE IF EXISTS payment_orders;
DROP TABLE IF EXISTS payment_channels;
