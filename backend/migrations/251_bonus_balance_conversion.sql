-- Schema only: installing this migration never converts any wallet.
CREATE TABLE bonus_balance_conversions (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    admin_user_id BIGINT NOT NULL,
    affected_users BIGINT NOT NULL DEFAULT 0,
    before_total NUMERIC NOT NULL DEFAULT 0,
    after_total NUMERIC NOT NULL DEFAULT 0,
    completed_at TIMESTAMPTZ
);
-- Retain exact before/after values independently of account deletion.
CREATE TABLE bonus_balance_conversion_users (
    user_id BIGINT PRIMARY KEY,
    balance_before NUMERIC(20,8) NOT NULL,
    balance_after NUMERIC(20,8) NOT NULL
);
CREATE OR REPLACE FUNCTION audit_bonus_balance_change() RETURNS TRIGGER LANGUAGE plpgsql AS $$
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
        IF category IS NULL OR category NOT IN ('signup','invite','checkin','recharge','consumption_reward','lottery','admin','consumption','conversion') THEN category := 'other'; END IF;
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

