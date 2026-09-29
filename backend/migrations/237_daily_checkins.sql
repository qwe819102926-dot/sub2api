CREATE TABLE IF NOT EXISTS user_daily_checkins (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checkin_date DATE NOT NULL,
    streak_day INTEGER NOT NULL CHECK (streak_day > 0),
    reward_amount NUMERIC(20,8) NOT NULL CHECK (reward_amount > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, checkin_date)
);

CREATE INDEX IF NOT EXISTS idx_user_daily_checkins_user_date
    ON user_daily_checkins(user_id, checkin_date DESC);
