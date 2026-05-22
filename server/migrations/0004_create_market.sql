-- +goose Up
CREATE TABLE orders (
    id             BIGSERIAL PRIMARY KEY,
    order_no       VARCHAR(64)  NOT NULL UNIQUE,
    buyer_user_id  BIGINT       NOT NULL REFERENCES users(id),
    item_id        BIGINT       NOT NULL REFERENCES catalog_items(id),
    plan_type      VARCHAR(32)  NOT NULL DEFAULT 'one_off',
    amount_credits BIGINT       NOT NULL,
    payment_method VARCHAR(64)  NOT NULL DEFAULT '',
    status         VARCHAR(32)  NOT NULL DEFAULT 'pending',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_buyer ON orders(buyer_user_id);
CREATE INDEX idx_orders_status ON orders(status);

CREATE TABLE reviews (
    id            BIGSERIAL PRIMARY KEY,
    buyer_user_id BIGINT       NOT NULL REFERENCES users(id),
    item_id       BIGINT       NOT NULL REFERENCES catalog_items(id),
    rating        SMALLINT     NOT NULL,
    body          TEXT         NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reviews_item ON reviews(item_id);

-- +goose Down
DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS orders;
