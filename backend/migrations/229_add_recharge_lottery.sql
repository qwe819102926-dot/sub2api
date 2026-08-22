CREATE TABLE IF NOT EXISTS recharge_lottery_entries (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL UNIQUE REFERENCES payment_orders(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    draw_count INTEGER NOT NULL CHECK (draw_count > 0),
    used_count INTEGER NOT NULL DEFAULT 0 CHECK (used_count >= 0 AND used_count <= draw_count),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recharge_lottery_entries_user_remaining
    ON recharge_lottery_entries(user_id)
    WHERE used_count < draw_count;

CREATE TABLE IF NOT EXISTS recharge_lottery_draws (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_id BIGINT NOT NULL REFERENCES recharge_lottery_entries(id) ON DELETE RESTRICT,
    order_id BIGINT NOT NULL REFERENCES payment_orders(id) ON DELETE RESTRICT,
    prize_amount NUMERIC(20,2) NOT NULL DEFAULT 0,
    is_winner BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recharge_lottery_draws_user_created
    ON recharge_lottery_draws(user_id, created_at DESC);
