-- Every grant has its own calendar-month lifetime. Wallet row locks serialize
-- grants, FIFO deductions and expiry; the original ledger remains immutable.
LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE;

CREATE TABLE bonus_balance_lots (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    grant_event_id BIGINT,
    date_estimated BOOLEAN NOT NULL DEFAULT FALSE,
    remaining NUMERIC(20,8) NOT NULL CHECK (remaining >= 0),
    granted_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_bonus_lots_user ON bonus_balance_lots(user_id, expires_at, id) WHERE remaining > 0;
CREATE INDEX idx_bonus_lots_expiry ON bonus_balance_lots(expires_at, user_id) WHERE remaining > 0;

CREATE FUNCTION bonus_balance_expires_at(granted TIMESTAMPTZ) RETURNS TIMESTAMPTZ
LANGUAGE sql IMMUTABLE STRICT AS $$
    SELECT ((granted AT TIME ZONE 'Asia/Shanghai') + INTERVAL '1 month') AT TIME ZONE 'Asia/Shanghai'
$$;

-- Reconstruct the unspent tail of grants (oldest grants were consumed first).
-- Normalize pre-conversion grants to the wallet's current units, retaining
-- their original dates. Never reset a grant's lifetime during a unit conversion.
DO $$
DECLARE wallet RECORD; grant_row RECORD; outstanding NUMERIC; take NUMERIC;
    cumulative NUMERIC; allocated NUMERIC; target NUMERIC; grant_time TIMESTAMPTZ;
BEGIN
    FOR wallet IN SELECT id, bonus_balance FROM users WHERE bonus_balance > 0 LOOP
        outstanding := wallet.bonus_balance;
        cumulative := 0; allocated := 0;
        FOR grant_row IN
            SELECT e.id, e.created_at, e.source='historical_credit' AS estimated,
                CASE WHEN c.user_id IS NOT NULL AND
                    e.id < COALESCE((SELECT MIN(x.id) FROM bonus_balance_events x
                        WHERE x.user_id=e.user_id AND x.source='conversion' AND x.amount<0), 9223372036854775807)
                    AND e.created_at <= r.completed_at
                THEN e.amount / 10 ELSE e.amount END AS amount
            FROM bonus_balance_events e
            LEFT JOIN bonus_balance_conversion_users c ON c.user_id=e.user_id
            LEFT JOIN bonus_balance_conversions r ON r.id=1 AND r.completed_at IS NOT NULL
            WHERE e.user_id=wallet.id AND e.amount>0 AND e.source<>'conversion'
            ORDER BY e.created_at DESC, e.id DESC
        LOOP
            cumulative := cumulative+grant_row.amount;
            target := ROUND(cumulative,8);
            take := LEAST(outstanding, target-allocated);
            allocated := target;
            IF take > 0 THEN
                -- Unknown imported dates receive the explicitly approved grace month.
                grant_time := CASE WHEN grant_row.estimated THEN statement_timestamp() ELSE grant_row.created_at END;
                INSERT INTO bonus_balance_lots(user_id,grant_event_id,date_estimated,remaining,granted_at,expires_at)
                VALUES(wallet.id,grant_row.id,grant_row.estimated,take,grant_time,bonus_balance_expires_at(grant_time));
                outstanding := outstanding-take;
            END IF;
            EXIT WHEN outstanding=0;
        END LOOP;
        IF outstanding <> 0 THEN
            INSERT INTO bonus_balance_lots(user_id,date_estimated,remaining,granted_at,expires_at)
            VALUES(wallet.id,TRUE,outstanding,statement_timestamp(),bonus_balance_expires_at(statement_timestamp()));
        END IF;
    END LOOP;
END;
$$;

-- Extend existing audit attribution without changing its aggregation rules.
DO $$
DECLARE definition TEXT;
BEGIN
    definition := pg_get_functiondef('audit_bonus_balance_change()'::regprocedure);
    definition := replace(definition, '''conversion'')', '''conversion'',''expiration'')');
    EXECUTE definition;
END;
$$;

CREATE FUNCTION sync_bonus_balance_lots() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE delta NUMERIC; needed NUMERIC; lot RECORD; take NUMERIC; category TEXT;
    cumulative NUMERIC := 0; allocated NUMERIC := 0; target NUMERIC;
BEGIN
    IF TG_OP='DELETE' THEN
        UPDATE bonus_balance_lots SET remaining=0 WHERE user_id=OLD.id AND remaining>0;
        RETURN NULL;
    END IF;
    IF TG_OP='INSERT' THEN delta:=COALESCE(NEW.bonus_balance,0);
    ELSE delta:=COALESCE(NEW.bonus_balance,0)-COALESCE(OLD.bonus_balance,0); END IF;
    IF delta=0 THEN RETURN NULL; END IF;
    category:=substring(current_query() FROM '/\* bonus_source:([a-z_]+) \*/');
    IF category='expiration' THEN RETURN NULL; END IF;
    IF delta>0 THEN
        INSERT INTO bonus_balance_lots(user_id,remaining,granted_at,expires_at)
        VALUES(NEW.id,delta,statement_timestamp(),bonus_balance_expires_at(statement_timestamp()));
    ELSIF category='conversion' AND OLD.bonus_balance>0 THEN
        -- Cumulative rounding preserves exactly the new eight-decimal total.
        FOR lot IN SELECT id,remaining FROM bonus_balance_lots WHERE user_id=NEW.id AND remaining>0 ORDER BY expires_at,id LOOP
            cumulative:=cumulative+lot.remaining;
            target:=ROUND(cumulative*NEW.bonus_balance/OLD.bonus_balance,8);
            UPDATE bonus_balance_lots SET remaining=target-allocated WHERE id=lot.id;
            allocated:=target;
        END LOOP;
    ELSE
        needed:=-delta;
        FOR lot IN SELECT id,remaining FROM bonus_balance_lots WHERE user_id=NEW.id AND remaining>0 ORDER BY expires_at,id LOOP
            take:=LEAST(needed,lot.remaining);
            UPDATE bonus_balance_lots SET remaining=remaining-take WHERE id=lot.id;
            needed:=needed-take;
            EXIT WHEN needed=0;
        END LOOP;
        IF needed<>0 THEN RAISE EXCEPTION 'Bonus lot balance mismatch for user %',NEW.id; END IF;
    END IF;
    RETURN NULL;
END;
$$;
CREATE TRIGGER bonus_lots_update AFTER UPDATE OF bonus_balance ON users FOR EACH ROW EXECUTE FUNCTION sync_bonus_balance_lots();
CREATE TRIGGER bonus_lots_insert AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION sync_bonus_balance_lots();
CREATE TRIGGER bonus_lots_delete AFTER DELETE ON users FOR EACH ROW EXECUTE FUNCTION sync_bonus_balance_lots();

-- Read paths reject expired units immediately, independently of the sweeper.
CREATE FUNCTION spendable_bonus_balance(owner_id BIGINT) RETURNS NUMERIC
LANGUAGE sql STABLE AS $$
    SELECT COALESCE(SUM(remaining),0) FROM bonus_balance_lots
    WHERE user_id=owner_id AND remaining>0 AND expires_at>statement_timestamp()
$$;

-- Call with bonus_source:expiration on the outer statement for audit tagging.
-- The row lock is retained until the billing transaction commits.
CREATE FUNCTION expire_user_bonus_balance(owner_id BIGINT) RETURNS NUMERIC LANGUAGE plpgsql AS $$
DECLARE wallet NUMERIC; expired NUMERIC; cutoff TIMESTAMPTZ;
BEGIN
    SELECT COALESCE(bonus_balance,0) INTO wallet FROM users WHERE id=owner_id FOR UPDATE;
    IF NOT FOUND THEN RETURN 0; END IF;
    -- A billing statement may have waited across the expiry boundary for this
    -- lock. Evaluate validity after acquiring it, not at statement start.
    cutoff := clock_timestamp();
    WITH cleared AS (
        UPDATE bonus_balance_lots SET remaining=0
        WHERE user_id=owner_id AND remaining>0 AND expires_at<=cutoff
        RETURNING id
    ) SELECT COALESCE(SUM(e.remaining),0) INTO expired FROM bonus_balance_lots e
      JOIN cleared c ON c.id=e.id;
    IF expired>0 THEN
        UPDATE users SET bonus_balance=bonus_balance-expired,updated_at=NOW() WHERE id=owner_id;
    END IF;
    RETURN wallet-expired;
END;
$$;

-- Apply the user's retrospective policy as part of this atomic migration.
/* bonus_source:expiration */ SELECT expire_user_bonus_balance(id) FROM users ORDER BY id;
