package repository

import (
	"context"
	"encoding/json"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const upstreamRecoveryEnabledSQL = "credentials @> '{\"upstream_failure_recovery_enabled\":true}'::jsonb"
const upstreamRecoveryStateSQL = "extra->'upstream_failure_recovery'"

func upstreamRecoverySchedulablePredicate() *entsql.Predicate {
	return entsql.P(func(b *entsql.Builder) {
		b.WriteString("NOT (").
			Ident(dbaccount.FieldCredentials).WriteString(" @> ").Arg("{\"upstream_failure_recovery_enabled\":true}").
			WriteString("::jsonb AND ").Ident(dbaccount.FieldExtra).
			WriteString("->'upstream_failure_recovery' IS NOT NULL)")
	})
}

func withoutUpstreamRecoveryState(extra map[string]any) map[string]any {
	if _, ok := extra[service.UpstreamRecoveryStateExtra]; !ok {
		return extra
	}
	copied := copyJSONMap(extra)
	delete(copied, service.UpstreamRecoveryStateExtra)
	return copied
}

func (r *accountRepository) BeginUpstreamRecovery(ctx context.Context, id int64, state service.UpstreamRecoveryState) error {
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return r.writeUpstreamRecoveryState(ctx, id, `
		UPDATE accounts
		SET extra = COALESCE(extra, '{}'::jsonb) || jsonb_build_object('upstream_failure_recovery', $2::jsonb)
		WHERE id = $1 AND deleted_at IS NULL AND status = 'active' AND schedulable
			AND `+upstreamRecoveryEnabledSQL+`
			AND `+upstreamRecoveryStateSQL+` IS NULL
	`, string(payload))
}

func (r *accountRepository) ClaimUpstreamRecovery(ctx context.Context, limit int, now, leaseUntil time.Time) ([]service.UpstreamRecoveryClaim, error) {
	rows, err := r.sql.QueryContext(ctx, `
		WITH due AS (
			SELECT id FROM accounts
			WHERE deleted_at IS NULL AND status = 'active' AND schedulable
				AND (NOT auto_pause_on_expired OR expires_at IS NULL OR expires_at > NOW())
				AND `+upstreamRecoveryEnabledSQL+`
				AND `+upstreamRecoveryStateSQL+` IS NOT NULL
				AND (`+upstreamRecoveryStateSQL+`->>'next_probe_at_ms')::bigint <= $1
				AND (`+upstreamRecoveryStateSQL+`->>'lease_until_ms')::bigint <= $1
			ORDER BY (`+upstreamRecoveryStateSQL+`->>'next_probe_at_ms')::bigint, id
			LIMIT $2 FOR UPDATE SKIP LOCKED
		)
		UPDATE accounts a
		SET extra = jsonb_set(a.extra, '{upstream_failure_recovery,lease_until_ms}', to_jsonb($3::bigint))
		FROM due WHERE a.id = due.id
		RETURNING a.id, a.extra->'upstream_failure_recovery'
	`, now.UnixMilli(), limit, leaseUntil.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var claims []service.UpstreamRecoveryClaim
	for rows.Next() {
		var claim service.UpstreamRecoveryClaim
		var state []byte
		if err := rows.Scan(&claim.AccountID, &state); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(state, &claim.State); err != nil {
			return nil, err
		}
		claims = append(claims, claim)
	}
	return claims, rows.Err()
}

func (r *accountRepository) FinishUpstreamRecovery(ctx context.Context, claim service.UpstreamRecoveryClaim, retryAt time.Time) error {
	const conditions = " WHERE id = $1 AND deleted_at IS NULL AND " + upstreamRecoveryEnabledSQL +
		" AND (" + upstreamRecoveryStateSQL + "->>'lease_until_ms')::bigint = $2"
	if retryAt.IsZero() {
		return r.writeUpstreamRecoveryState(ctx, claim.AccountID,
			"UPDATE accounts SET extra = extra - 'upstream_failure_recovery'"+conditions, claim.State.LeaseUntilMS)
	}
	return r.writeUpstreamRecoveryState(ctx, claim.AccountID,
		"UPDATE accounts SET extra = jsonb_set(jsonb_set(extra, '{upstream_failure_recovery,next_probe_at_ms}',"+
			" to_jsonb($3::bigint)), '{upstream_failure_recovery,lease_until_ms}', '0'::jsonb)"+conditions,
		claim.State.LeaseUntilMS, retryAt.UnixMilli())
}

func (r *accountRepository) writeUpstreamRecoveryState(ctx context.Context, id int64, query string, args ...any) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	client := tx.Client()
	result, err := client.ExecContext(ctx, query, append([]any{id}, args...)...)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return err
	}
	if err := enqueueSchedulerOutbox(ctx, client, service.SchedulerOutboxEventAccountChanged, &id, nil, nil); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	r.syncSchedulerAccountSnapshot(ctx, id)
	return nil
}
