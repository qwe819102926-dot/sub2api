CREATE TABLE IF NOT EXISTS recharge_bonus_grants (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL UNIQUE REFERENCES payment_orders(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    threshold NUMERIC(20,2) NOT NULL,
    bonus_amount NUMERIC(20,2) NOT NULL CHECK (bonus_amount > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS consumption_reward_claims (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    threshold NUMERIC(20,2) NOT NULL,
    bonus_amount NUMERIC(20,2) NOT NULL CHECK (bonus_amount > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, threshold)
);

CREATE INDEX IF NOT EXISTS idx_consumption_reward_claims_user
    ON consumption_reward_claims(user_id, created_at DESC);

ALTER TABLE users ADD COLUMN IF NOT EXISTS bonus_balance DECIMAL(20,8) NOT NULL DEFAULT 0;
