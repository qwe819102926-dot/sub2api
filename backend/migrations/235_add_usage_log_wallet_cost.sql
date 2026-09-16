-- usage_logs.wallet_cost 记录本金余额实扣，供用户仪表盘「今日消费 / 累计消费」汇总。
-- 赠送余额实扣不写入此列；用户使用记录仍用 actual_cost 展示账单价。
-- NULL 表示历史行，统计时回退 actual_cost。
-- 可空且无 DEFAULT，分区表上是 metadata-only 加列。
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS wallet_cost DECIMAL(20, 10);

COMMENT ON COLUMN usage_logs.wallet_cost IS
    'Principal wallet units actually deducted; bonus-balance deductions are excluded. NULL means historical rows should fall back to actual_cost.';
