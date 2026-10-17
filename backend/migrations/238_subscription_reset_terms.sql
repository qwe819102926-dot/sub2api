ALTER TABLE user_subscriptions
    ADD COLUMN IF NOT EXISTS base_validity_days integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS source_plan_id bigint;

-- Recover the original purchased term from the order recorded on this subscription.
-- Never infer the base term from expires_at - starts_at: renewals may have added days.
WITH originals AS (
    SELECT DISTINCT ON (s.id) s.id, o.plan_id, o.subscription_days
    FROM user_subscriptions s
    JOIN payment_orders o ON o.user_id = s.user_id
        AND o.subscription_group_id = s.group_id
        AND o.order_type = 'subscription' AND o.status = 'COMPLETED'
        AND o.subscription_days BETWEEN 1 AND 36500
        AND position(E'\npayment order ' || o.id::text || E'\n'
            IN E'\n' || replace(coalesce(s.notes, ''), E'\r\n', E'\n') || E'\n') > 0
    WHERE s.base_validity_days = 0
    ORDER BY s.id, o.created_at, o.id
)
UPDATE user_subscriptions s
SET base_validity_days = o.subscription_days, source_plan_id = o.plan_id
FROM originals o WHERE s.id = o.id;

-- Old manually assigned subscriptions can use the only sale plan for their group.
-- Ambiguous groups remain unbound and cannot create reset orders.
WITH only_plans AS (
    SELECT group_id, min(id) AS plan_id,
        min(CASE WHEN validity_unit IN ('week', 'weeks') THEN validity_days * 7
            WHEN validity_unit IN ('month', 'months') THEN validity_days * 30
            ELSE validity_days END) AS days
    FROM subscription_plans WHERE for_sale = true
    GROUP BY group_id HAVING count(*) = 1
)
UPDATE user_subscriptions s
SET base_validity_days = p.days, source_plan_id = p.plan_id
FROM only_plans p
WHERE s.group_id = p.group_id AND s.base_validity_days = 0
    AND p.days BETWEEN 1 AND 36500;
