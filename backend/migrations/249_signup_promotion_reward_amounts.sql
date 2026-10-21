-- Apply the new reward amounts once. Existing grants and balances stay intact.
INSERT INTO settings (key, value, updated_at) VALUES
    ('signup_bonus_amount', '10', NOW()),
    ('invite_bonus_amount', '10', NOW())
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at;
