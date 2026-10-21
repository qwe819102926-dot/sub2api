-- Freeze wallet mutations while constructing the historical opening ledger.
LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE;

CREATE TABLE bonus_balance_events (
    id BIGSERIAL PRIMARY KEY,
    event_key TEXT UNIQUE,
    user_id BIGINT NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL,
    amount NUMERIC(20,8) NOT NULL CHECK (amount <> 0),
    balance_after NUMERIC(20,8),
    historical BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- No cascading FK: removing a user or a campaign must retain the audit trail.
CREATE INDEX idx_bonus_balance_events_user ON bonus_balance_events(user_id, created_at DESC, id DESC);
CREATE INDEX idx_bonus_balance_events_source ON bonus_balance_events(source, created_at DESC, id DESC);
CREATE INDEX idx_bonus_balance_events_time ON bonus_balance_events(created_at DESC, id DESC);

INSERT INTO bonus_balance_events(user_id, email, source, amount, historical, created_at)
SELECT g.user_id, COALESCE(u.email, ''), g.source, g.amount, TRUE, g.created_at
FROM (
    SELECT user_id, kind AS source, amount, created_at FROM signup_promotion_grants
    UNION ALL SELECT user_id, 'checkin', reward_amount, created_at FROM user_daily_checkins
    UNION ALL SELECT user_id, 'recharge', bonus_amount, created_at FROM recharge_bonus_grants
    UNION ALL SELECT user_id, 'consumption_reward', bonus_amount, created_at FROM consumption_reward_claims
    -- Before migration 231, lottery prizes were credited to principal balance.
    UNION ALL SELECT user_id, 'lottery', prize_amount, created_at FROM recharge_lottery_draws
        WHERE is_winner AND prize_amount > 0 AND created_at >= (SELECT applied_at FROM schema_migrations WHERE filename = '231_add_reward_campaigns.sql')
    UNION ALL SELECT used_by, 'admin', value, COALESCE(used_at, created_at) FROM redeem_codes WHERE type = 'admin_bonus' AND status = 'used' AND used_by IS NOT NULL AND value <> 0
) g LEFT JOIN users u ON u.id = g.user_id WHERE g.amount <> 0;

-- Import only measured bonus-wallet debits. NULL bonus_cost rows cannot be
-- converted reliably because historical consumption multipliers are unknown.
INSERT INTO bonus_balance_events(user_id, email, source, amount, historical)
SELECT l.user_id, COALESCE(u.email, ''), 'consumption', -ROUND(SUM(l.bonus_cost), 8), TRUE
FROM usage_logs l LEFT JOIN users u ON u.id = l.user_id
WHERE l.bonus_cost > 0 GROUP BY l.user_id, u.email HAVING ROUND(SUM(l.bonus_cost), 8) > 0;

-- Reconcile each retained wallet without claiming unknown outflows were usage.
WITH recorded AS (SELECT user_id, SUM(amount) AS balance FROM bonus_balance_events GROUP BY user_id)
INSERT INTO bonus_balance_events(user_id, email, source, amount, balance_after, historical)
SELECT COALESCE(u.id, r.user_id), COALESCE(u.email, ''),
       CASE WHEN COALESCE(u.bonus_balance, 0) - COALESCE(r.balance, 0) > 0 THEN 'historical_credit' ELSE 'historical_outflow' END,
       COALESCE(u.bonus_balance, 0) - COALESCE(r.balance, 0), COALESCE(u.bonus_balance, 0), TRUE
FROM users u FULL JOIN recorded r ON r.user_id = u.id
WHERE COALESCE(u.bonus_balance, 0) <> COALESCE(r.balance, 0);

UPDATE bonus_balance_events SET updated_at = created_at WHERE historical;

CREATE FUNCTION audit_bonus_balance_change() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    delta NUMERIC(20,8);
    category TEXT;
    owner_id BIGINT;
    owner_email TEXT;
    after_balance NUMERIC(20,8);
    bucket TEXT;
BEGIN
    IF TG_OP = 'DELETE' THEN
        delta := -COALESCE(OLD.bonus_balance, 0);
        owner_id := OLD.id; owner_email := OLD.email; after_balance := 0;
        category := 'account_removed';
    ELSE
        owner_id := NEW.id; owner_email := NEW.email; after_balance := COALESCE(NEW.bonus_balance, 0);
        IF TG_OP = 'INSERT' THEN delta := after_balance;
        ELSE delta := after_balance - COALESCE(OLD.bonus_balance, 0); END IF;
        -- Explicit tags keep accounting attribution in the same SQL statement
        -- and transaction as the balance change, without session-state leakage.
        category := substring(current_query() FROM '/\* bonus_source:([a-z_]+) \*/');
        IF category IS NULL OR category NOT IN ('signup','invite','checkin','recharge','consumption_reward','lottery','admin','consumption') THEN category := 'other'; END IF;
        IF category = 'consumption' AND delta > 0 THEN category := 'other'; END IF;
    END IF;
    IF delta = 0 THEN RETURN NULL; END IF;
    -- Consumption rows aggregate per user/business day; accounting survives
    -- usage-log cleanup without adding one permanent audit row per request.
    IF category = 'consumption' THEN
        bucket := 'usage:' || owner_id || ':' || (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Shanghai')::date;
    ELSE bucket := NULL; END IF;
    INSERT INTO bonus_balance_events(event_key, user_id, email, source, amount, balance_after)
    VALUES (bucket, owner_id, COALESCE(owner_email, ''), category, delta, after_balance)
    ON CONFLICT (event_key) DO UPDATE
    SET amount = bonus_balance_events.amount + EXCLUDED.amount, balance_after = EXCLUDED.balance_after, email = EXCLUDED.email, updated_at = NOW();
    RETURN NULL;
END;
$$;

CREATE TRIGGER bonus_balance_audit_update AFTER UPDATE OF bonus_balance ON users FOR EACH ROW EXECUTE FUNCTION audit_bonus_balance_change();
CREATE TRIGGER bonus_balance_audit_insert AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION audit_bonus_balance_change();
CREATE TRIGGER bonus_balance_audit_delete AFTER DELETE ON users FOR EACH ROW EXECUTE FUNCTION audit_bonus_balance_change();
