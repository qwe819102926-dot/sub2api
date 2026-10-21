-- Update future signup and invitation grants; retain historical rewards.
INSERT INTO settings (key, value, updated_at) VALUES
    ('signup_bonus_amount', '20', NOW()),
    ('invite_bonus_amount', '20', NOW())
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at;
