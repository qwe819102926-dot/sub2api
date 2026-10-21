package service

import (
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SettingKeySignupBonusEnabled = "signup_bonus_enabled"
	SettingKeySignupBonusAmount  = "signup_bonus_amount"
	SettingKeyInviteBonusEnabled = "invite_bonus_enabled"
	SettingKeyInviteBonusAmount  = "invite_bonus_amount"
)

type SignupPromotionConfig struct {
	SignupBonusEnabled bool    `json:"signup_bonus_enabled"`
	SignupBonusAmount  float64 `json:"signup_bonus_amount"`
	InviteBonusEnabled bool    `json:"invite_bonus_enabled"`
	InviteBonusAmount  float64 `json:"invite_bonus_amount"`
}

type SignupPromotionRecord struct {
	UserID    int64     `json:"user_id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	Amount    float64   `json:"amount"`
	Status    string    `json:"status"`
}

type SignupPromotionDetail struct {
	SignupPromotionConfig
	AffCode           string                  `json:"aff_code"`
	InvitedCount      int64                   `json:"invited_count"`
	TotalBonus        float64                 `json:"total_bonus"`
	Page              int                     `json:"page"`
	PageSize          int                     `json:"page_size"`
	Records           []SignupPromotionRecord `json:"records"`
	LegacyQuota       float64                 `json:"legacy_quota"`
	LegacyFrozenQuota float64                 `json:"legacy_frozen_quota"`
}

// Kept separate from AffiliateRepository: registration gifts and recharge
// commissions have independent eligibility and accounting.
type SignupPromotionRepository interface {
	ApplySignupPromotion(ctx context.Context, userID int64, code string, config SignupPromotionConfig) ([]int64, error)
	GetSignupPromotionDetail(ctx context.Context, userID int64, page, pageSize int) (*SignupPromotionDetail, error)
}

func (s *SettingService) GetSignupPromotionConfig(ctx context.Context) (SignupPromotionConfig, error) {
	if s == nil || s.settingRepo == nil {
		return SignupPromotionConfig{}, nil
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeySignupBonusEnabled, SettingKeySignupBonusAmount, SettingKeyInviteBonusEnabled, SettingKeyInviteBonusAmount})
	if err != nil {
		return SignupPromotionConfig{}, err
	}
	return promotionConfigFromValues(values), nil
}

func promotionConfigFromValues(values map[string]string) SignupPromotionConfig {
	amount := func(key string) float64 {
		v, err := strconv.ParseFloat(values[key], 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1000000 {
			return 10
		}
		return QuantizeUsageBillingAmount(v)
	}
	return SignupPromotionConfig{
		SignupBonusEnabled: values[SettingKeySignupBonusEnabled] == "true",
		SignupBonusAmount:  amount(SettingKeySignupBonusAmount),
		InviteBonusEnabled: values[SettingKeyInviteBonusEnabled] == "true",
		InviteBonusAmount:  amount(SettingKeyInviteBonusAmount),
	}
}

func (s *AffiliateService) ApplySignupPromotion(ctx context.Context, userID int64, rawCode string) error {
	if s == nil {
		return nil
	}
	config, err := s.settingService.GetSignupPromotionConfig(ctx)
	if err != nil {
		return err
	}
	if !config.SignupBonusEnabled && !config.InviteBonusEnabled {
		return nil
	}
	repo, ok := s.repo.(SignupPromotionRepository)
	if !ok {
		return infraerrors.ServiceUnavailable("PROMOTION_UNAVAILABLE", "signup promotion unavailable")
	}
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if !isValidAffiliateCodeFormat(code) {
		code = ""
	}
	userIDs, err := repo.ApplySignupPromotion(ctx, userID, code, config)
	if err != nil {
		return err
	}
	invalidate := func() {
		for _, id := range userIDs {
			s.invalidateAffiliateCaches(context.WithoutCancel(ctx), id)
		}
	}
	if tx := dbent.TxFromContext(ctx); tx != nil {
		tx.OnCommit(func(next dbent.Committer) dbent.Committer {
			return dbent.CommitFunc(func(commitCtx context.Context, tx *dbent.Tx) error {
				if err := next.Commit(commitCtx, tx); err != nil {
					return err
				}
				invalidate()
				return nil
			})
		})
	} else {
		invalidate()
	}
	return nil
}

func (s *AffiliateService) GetSignupPromotionDetail(ctx context.Context, userID int64, page, pageSize int) (*SignupPromotionDetail, error) {
	if s == nil {
		return nil, ErrServiceUnavailable
	}
	repo, ok := s.repo.(SignupPromotionRepository)
	if !ok {
		return nil, ErrServiceUnavailable
	}
	if page < 1 {
		page = 1
	}
	if page > 1000000 {
		page = 1000000
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	config, err := s.settingService.GetSignupPromotionConfig(ctx)
	if err != nil {
		return nil, err
	}
	detail, err := repo.GetSignupPromotionDetail(ctx, userID, page, pageSize)
	if err != nil {
		return nil, err
	}
	detail.SignupPromotionConfig = config
	for i := range detail.Records {
		detail.Records[i].Email = maskEmail(detail.Records[i].Email)
	}
	// Legacy claims remain usable when the old commission switch is off.
	if _, err := s.repo.ThawFrozenQuota(ctx, userID); err != nil {
		return nil, err
	}
	legacy, err := s.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return nil, err
	}
	detail.LegacyQuota, detail.LegacyFrozenQuota = legacy.AffQuota, legacy.AffFrozenQuota
	return detail, nil
}
