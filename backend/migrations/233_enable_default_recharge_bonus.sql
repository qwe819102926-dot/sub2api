-- Keep the recharge bonus campaign active with a default matching tier.
INSERT INTO settings (key, value, updated_at)
VALUES
    ('RECHARGE_BONUS_ENABLED', 'true', NOW()),
    ('RECHARGE_BONUS_TIERS', '[{"threshold":10,"bonus":1}]', NOW())
ON CONFLICT (key) DO UPDATE
SET value = EXCLUDED.value,
    updated_at = EXCLUDED.updated_at;
