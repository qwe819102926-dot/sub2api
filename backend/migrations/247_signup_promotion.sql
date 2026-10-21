-- Only completed registrations after this migration participate. Legacy affiliate
-- quotas and ledger entries remain available for thawing and transfer.
CREATE TABLE signup_promotions (
    user_id BIGINT PRIMARY KEY REFERENCES users(id),
    inviter_id BIGINT REFERENCES users(id),
    signup_bonus NUMERIC(20,8) NOT NULL CHECK (signup_bonus >= 0),
    invite_bonus NUMERIC(20,8) NOT NULL CHECK (invite_bonus >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (inviter_id IS NULL OR inviter_id <> user_id)
);
CREATE INDEX idx_signup_promotions_inviter ON signup_promotions(inviter_id, created_at DESC, user_id DESC);

CREATE TABLE signup_promotion_grants (
    id BIGSERIAL PRIMARY KEY,
    source_user_id BIGINT NOT NULL REFERENCES signup_promotions(user_id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    kind TEXT NOT NULL CHECK (kind IN ('signup', 'invite')),
    amount NUMERIC(20,8) NOT NULL CHECK (amount > 0),
    bonus_balance_after NUMERIC(20,8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(source_user_id, kind)
);
CREATE INDEX idx_signup_promotion_grants_user ON signup_promotion_grants(user_id, created_at DESC);

INSERT INTO settings (key, value, updated_at) VALUES
    ('signup_bonus_enabled', 'true', NOW()),
    ('signup_bonus_amount', '50', NOW()),
    ('invite_bonus_enabled', 'true', NOW()),
    ('invite_bonus_amount', '50', NOW())
ON CONFLICT (key) DO NOTHING;

-- Stop new recharge commissions without changing any existing entitlement.
UPDATE settings SET value = 'false', updated_at = NOW() WHERE key = 'affiliate_enabled';
