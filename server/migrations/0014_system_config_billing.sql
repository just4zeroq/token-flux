-- +goose Up
-- +goose StatementBegin
-- Billing exchange rates: how many credits per unit of fiat currency.
INSERT INTO system_configs (key, value, description) VALUES
    ('billing.credits_per_cny', '10', 'Number of credits purchased for 1 CNY'),
    ('billing.credits_per_usd', '70', 'Number of credits purchased for 1 USD')
ON CONFLICT (key) DO NOTHING;

-- Reward points per credit consumed/earned (users get points when they spend
-- credits; providers get points when they earn revenue credits).
INSERT INTO system_configs (key, value, description) VALUES
    ('billing.points_per_credit_consumer', '1', 'Reward points awarded to consumer per 1 credit spent'),
    ('billing.points_per_credit_provider', '1', 'Reward points awarded to provider per 1 credit earned')
ON CONFLICT (key) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM system_configs WHERE key IN (
    'billing.credits_per_cny',
    'billing.credits_per_usd',
    'billing.points_per_credit_consumer',
    'billing.points_per_credit_provider'
);
-- +goose StatementEnd
