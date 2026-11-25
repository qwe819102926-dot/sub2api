CREATE INDEX IF NOT EXISTS idx_accounts_upstream_recovery_due
    ON accounts (((extra->'upstream_failure_recovery'->>'next_probe_at_ms')::bigint), id)
    WHERE deleted_at IS NULL
        AND status = 'active'
        AND schedulable
        AND credentials @> '{"upstream_failure_recovery_enabled":true}'::jsonb
        AND extra->'upstream_failure_recovery' IS NOT NULL;
