-- Only successfully committed new accounts have an IP registration event.
-- No FK: account removal must not erase registration-abuse evidence.
CREATE TABLE IF NOT EXISTS registration_ip_events (
 user_id BIGINT PRIMARY KEY, ip INET NOT NULL, email TEXT NOT NULL,
 source TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS idx_registration_ip_events_ip_time ON registration_ip_events(ip, created_at DESC);
CREATE TABLE IF NOT EXISTS registration_ip_blocks (
 ip INET PRIMARY KEY, blocked_at TIMESTAMPTZ NOT NULL,
 blocked_until TIMESTAMPTZ NOT NULL, registrations INTEGER NOT NULL,
 ban_count INTEGER NOT NULL DEFAULT 1, reset_at TIMESTAMPTZ
);
INSERT INTO settings(key,value,updated_at) VALUES
 ('registration_ip_monitor','{"enabled":true,"window_hours":24,"max_accounts":5,"ban_hours":24}',NOW())
ON CONFLICT(key) DO NOTHING;
