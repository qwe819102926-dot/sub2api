-- Establish a durable reporting boundary without changing wallets or old events.
-- Wait for in-flight wallet writes before taking the event watermark. Event IDs
-- avoid misclassifying transactions whose NOW() predates the migration.
LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE bonus_balance_events IN SHARE ROW EXCLUSIVE MODE;
CREATE TABLE IF NOT EXISTS bonus_balance_reporting_epoch (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    started_at TIMESTAMPTZ NOT NULL,
    start_event_id BIGINT NOT NULL
);
INSERT INTO bonus_balance_reporting_epoch(id, started_at, start_event_id)
SELECT 1, clock_timestamp(), COALESCE(MAX(id), 0) FROM bonus_balance_events
ON CONFLICT (id) DO NOTHING;
