-- +goose Up
-- +goose StatementBegin
ALTER TABLE system_configs ADD COLUMN IF NOT EXISTS name VARCHAR(256) NOT NULL DEFAULT '';

-- Backfill names for existing configs.
UPDATE system_configs SET name = '最大重试次数'             WHERE key = 'gateway.max_retries' AND name = '';
UPDATE system_configs SET name = '接口默认速率'             WHERE key = 'gateway.rate_limit_default_qps' AND name = '';
UPDATE system_configs SET name = '人民币兑换汇率'           WHERE key = 'billing.credits_per_cny' AND name = '';
UPDATE system_configs SET name = '美元兑换汇率'             WHERE key = 'billing.credits_per_usd' AND name = '';
UPDATE system_configs SET name = '用户消耗积分奖励'         WHERE key = 'billing.points_per_credit_consumer' AND name = '';
UPDATE system_configs SET name = '提供商获取积分奖励'        WHERE key = 'billing.points_per_credit_provider' AND name = '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE system_configs DROP COLUMN IF EXISTS name;
-- +goose StatementEnd
