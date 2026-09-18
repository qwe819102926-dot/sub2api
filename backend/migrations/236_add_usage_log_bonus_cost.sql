-- usage_logs.bonus_cost records bonus-balance units actually deducted,
-- including any bonus consumption multiplier, for the admin profit card.
-- Usage records still use actual_cost for billed amounts.
-- NULL means historical rows; stats fall back to billed bonus-equivalent
-- GREATEST(actual_cost - COALESCE(wallet_cost, actual_cost), 0).
-- Nullable with no DEFAULT so partitioned tables get a metadata-only add.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS bonus_cost DECIMAL(20, 10);

COMMENT ON COLUMN usage_logs.bonus_cost IS
    'Bonus wallet units actually deducted, including consumption multipliers. NULL means historical rows should fall back to billed bonus-equivalent.';
