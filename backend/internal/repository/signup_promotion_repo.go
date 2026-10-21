package repository

import (
	"context"
	"errors"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.SignupPromotionRepository = (*affiliateRepository)(nil)

func (r *affiliateRepository) ApplySignupPromotion(ctx context.Context, userID int64, code string, config service.SignupPromotionConfig) ([]int64, error) {
	var credited []int64
	err := r.withTx(ctx, func(txCtx context.Context, client *dbent.Client) error {
		var inviterID *int64
		if config.InviteBonusEnabled && code != "" {
			rows, err := client.QueryContext(txCtx, `SELECT ua.user_id FROM user_affiliates ua JOIN users u ON u.id = ua.user_id WHERE ua.aff_code = $1 AND ua.user_id <> $2 AND u.deleted_at IS NULL AND u.status = 'active'`, code, userID)
			if err != nil {
				return err
			}
			if rows.Next() {
				var id int64
				if err := rows.Scan(&id); err != nil {
					return errors.Join(err, rows.Close())
				}
				inviterID = &id
			}
			if err := errors.Join(rows.Err(), rows.Close()); err != nil {
				return err
			}
		}
		signupBonus, inviteBonus := 0.0, 0.0
		if config.SignupBonusEnabled {
			signupBonus = config.SignupBonusAmount
		}
		if inviterID != nil {
			inviteBonus = config.InviteBonusAmount
		}
		result, err := client.ExecContext(txCtx, `INSERT INTO signup_promotions(user_id, inviter_id, signup_bonus, invite_bonus) VALUES ($1, $2, $3, $4) ON CONFLICT (user_id) DO NOTHING`, userID, inviterID, signupBonus, inviteBonus)
		if err != nil {
			return err
		}
		count, err := result.RowsAffected()
		if err != nil || count == 0 {
			return err
		}
		if inviterID != nil {
			if _, err := ensureUserAffiliateWithClient(txCtx, client, userID); err != nil {
				return err
			}
			// First binding wins. A mismatched binding is never eligible for a gift.
			result, err := client.ExecContext(txCtx, `UPDATE user_affiliates SET inviter_id = $1, updated_at = NOW() WHERE user_id = $2 AND inviter_id IS NULL`, *inviterID, userID)
			if err != nil {
				return err
			}
			bound, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if bound == 0 {
				return fmt.Errorf("promotion inviter already bound")
			}
			if _, err := client.ExecContext(txCtx, `UPDATE user_affiliates SET aff_count = aff_count + 1, updated_at = NOW() WHERE user_id = $1`, *inviterID); err != nil {
				return err
			}
		}
		for _, grant := range []struct {
			userID int64
			kind   string
			amount float64
		}{
			{userID, "signup", signupBonus},
			{promotionInviterID(inviterID), "invite", inviteBonus},
		} {
			if grant.amount <= 0 || grant.userID <= 0 {
				continue
			}
			if err := creditSignupPromotion(txCtx, client, userID, grant.userID, grant.kind, grant.amount); err != nil {
				return err
			}
			credited = append(credited, grant.userID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return credited, nil
}

func promotionInviterID(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}

// Balance and grant are written in one transaction; the source registration's
// primary key serializes concurrent retries before either balance is touched.
func creditSignupPromotion(ctx context.Context, client affiliateQueryExecer, sourceID, userID int64, kind string, amount float64) error {
	source := "signup"
	if kind == "invite" {
		source = "invite"
	}
	result, err := client.ExecContext(ctx, "/* bonus_source:"+source+" */ "+`WITH credited AS (
UPDATE users SET bonus_balance = COALESCE(bonus_balance, 0) + $1, updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL AND status = 'active'
RETURNING bonus_balance
)
INSERT INTO signup_promotion_grants(source_user_id, user_id, kind, amount, bonus_balance_after)
SELECT $3, $2, $4, $1, bonus_balance FROM credited`, amount, userID, sourceID, kind)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return service.ErrUserNotFound
	}
	return nil
}

func (r *affiliateRepository) GetSignupPromotionDetail(ctx context.Context, userID int64, page, pageSize int) (*service.SignupPromotionDetail, error) {
	summary, err := r.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return nil, err
	}
	detail := &service.SignupPromotionDetail{AffCode: summary.AffCode, Page: page, PageSize: pageSize, Records: []service.SignupPromotionRecord{}}
	client := clientFromContext(ctx, r.client)
	rows, err := client.QueryContext(ctx, `SELECT COUNT(*), COALESCE(SUM(g.amount), 0)::double precision FROM signup_promotions p LEFT JOIN signup_promotion_grants g ON g.source_user_id = p.user_id AND g.kind = 'invite' AND g.user_id = p.inviter_id WHERE p.inviter_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		err = rows.Scan(&detail.InvitedCount, &detail.TotalBonus)
	}
	if err == nil {
		err = rows.Err()
	}
	err = errors.Join(err, rows.Close())
	if err != nil {
		return nil, err
	}
	rows, err = client.QueryContext(ctx, `SELECT p.user_id, COALESCE(u.email, ''), p.created_at, COALESCE(g.amount, 0)::double precision,
CASE WHEN g.id IS NOT NULL THEN 'credited' ELSE 'no_reward' END
FROM signup_promotions p JOIN users u ON u.id = p.user_id
LEFT JOIN signup_promotion_grants g ON g.source_user_id = p.user_id AND g.kind = 'invite' AND g.user_id = p.inviter_id
WHERE p.inviter_id = $1 ORDER BY p.created_at DESC, p.user_id DESC LIMIT $2 OFFSET $3`, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var record service.SignupPromotionRecord
		if err := rows.Scan(&record.UserID, &record.Email, &record.CreatedAt, &record.Amount, &record.Status); err != nil {
			return nil, err
		}
		detail.Records = append(detail.Records, record)
	}
	return detail, errors.Join(rows.Err(), rows.Close())
}
