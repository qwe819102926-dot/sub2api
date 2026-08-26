-- Enable the default cumulative-spend campaign requested by the user.
INSERT INTO settings (key, value, updated_at)
VALUES
    ('BALANCE_RECHARGE_MULTIPLIER', '1.10', NOW()),
    ('CONSUMPTION_REWARD_ENABLED', 'true', NOW()),
    ('CONSUMPTION_REWARD_TIERS', '[{"threshold":20,"bonus":3},{"threshold":50,"bonus":5},{"threshold":150,"bonus":10},{"threshold":200,"bonus":13},{"threshold":300,"bonus":15},{"threshold":500,"bonus":20}]', NOW())
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value,
    updated_at = EXCLUDED.updated_at;
