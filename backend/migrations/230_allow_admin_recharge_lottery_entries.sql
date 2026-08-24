-- Allow administrators to grant lottery chances without a payment order.
-- Existing recharge entries retain their order association.
ALTER TABLE recharge_lottery_entries
    ALTER COLUMN order_id DROP NOT NULL;

ALTER TABLE recharge_lottery_draws
    ALTER COLUMN order_id DROP NOT NULL;
