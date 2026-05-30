-- +goose Up
CREATE TABLE invoices (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL DEFAULT 0,
    amount_credits  BIGINT NOT NULL DEFAULT 0,
    ref_type        VARCHAR(64) NOT NULL DEFAULT '',
    ref_id          BIGINT NOT NULL DEFAULT 0,
    invoice_number  VARCHAR(64) NOT NULL DEFAULT '',
    status          VARCHAR(32) NOT NULL DEFAULT 'pending',
    notes           TEXT NOT NULL DEFAULT '',
    issued_at       TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_invoices_user ON invoices(user_id, created_at);
CREATE INDEX idx_invoices_status ON invoices(status, created_at);

-- +goose Down
DROP TABLE IF EXISTS invoices;
