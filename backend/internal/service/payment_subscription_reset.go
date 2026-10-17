package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type SubscriptionResetQuote struct {
	SubscriptionID int64   `json:"subscription_id"`
	PlanID         int64   `json:"plan_id"`
	GroupID        int64   `json:"group_id"`
	PlanName       string  `json:"plan_name"`
	Price          float64 `json:"price"`
	ValidityDays   int     `json:"validity_days"`
}

func (s *PaymentService) GetSubscriptionResetQuote(ctx context.Context, userID, subscriptionID int64) (*SubscriptionResetQuote, error) {
	if s.subscriptionSvc == nil {
		return nil, infraerrors.ServiceUnavailable("SUBSCRIPTION_RESET_UNAVAILABLE", "subscription service is unavailable")
	}
	sub, err := s.subscriptionSvc.GetByID(ctx, subscriptionID)
	if err != nil || sub == nil || sub.UserID != userID || sub.DeletedAt != nil {
		return nil, infraerrors.NotFound("SUBSCRIPTION_NOT_FOUND", "subscription not found")
	}
	if sub.Status != SubscriptionStatusActive || !sub.ExpiresAt.After(time.Now()) {
		return nil, infraerrors.BadRequest("SUBSCRIPTION_RESET_UNAVAILABLE", "only active subscriptions can be reset")
	}
	if sub.BaseValidityDays < 1 || sub.BaseValidityDays > MaxValidityDays {
		return nil, infraerrors.BadRequest("SUBSCRIPTION_TERM_UNKNOWN", "original subscription term is unavailable; please contact the administrator")
	}
	var plan *dbent.SubscriptionPlan
	if sub.SourcePlanID != nil {
		plan, err = s.configService.GetPlan(ctx, *sub.SourcePlanID)
		if err != nil {
			return nil, infraerrors.BadRequest("RESET_PLAN_UNAVAILABLE", "original subscription plan is no longer available")
		}
	} else {
		plans, listErr := s.configService.ListPlansForSale(ctx)
		if listErr != nil {
			return nil, listErr
		}
		for _, candidate := range plans {
			if candidate.GroupID == sub.GroupID && psComputeValidityDays(candidate.ValidityDays, candidate.ValidityUnit) == sub.BaseValidityDays {
				if plan != nil {
					return nil, infraerrors.BadRequest("RESET_PLAN_AMBIGUOUS", "multiple plans match this subscription; please contact the administrator")
				}
				plan = candidate
			}
		}
	}
	if plan == nil || !plan.ForSale || plan.GroupID != sub.GroupID || plan.Price <= 0 || math.IsNaN(plan.Price) || math.IsInf(plan.Price, 0) {
		return nil, infraerrors.BadRequest("RESET_PLAN_UNAVAILABLE", "a paid subscription plan is not available for reset")
	}
	group, err := s.groupRepo.GetByID(ctx, sub.GroupID)
	if err != nil || group.Status != payment.EntityStatusActive || !group.IsSubscriptionType() {
		return nil, infraerrors.BadRequest("RESET_PLAN_UNAVAILABLE", "subscription group is unavailable")
	}
	return &SubscriptionResetQuote{SubscriptionID: sub.ID, PlanID: plan.ID, GroupID: sub.GroupID,
		PlanName: plan.Name, Price: plan.Price, ValidityDays: sub.BaseValidityDays}, nil
}

func IsSubscriptionResetOrder(order *dbent.PaymentOrder) bool {
	return order != nil && order.OrderType == payment.OrderTypeSubscription && order.ProviderSnapshot["subscription_action"] == "reset"
}

func subscriptionResetTarget(order *dbent.PaymentOrder) (int64, error) {
	value := order.ProviderSnapshot["subscription_reset_id"]
	var id int64
	switch v := value.(type) {
	case int64:
		id = v
	case int:
		id = int64(v)
	case float64:
		if v > 0 && v < float64(math.MaxInt64) && v == math.Trunc(v) {
			id = int64(v)
		}
	case json.Number:
		id, _ = v.Int64()
	}
	if id <= 0 {
		return 0, fmt.Errorf("invalid reset subscription snapshot")
	}
	return id, nil
}

// Keep the reset and the existing fulfillment audit in one transaction. Locking
// the order before checking the audit makes retries and concurrent callbacks safe.
func (s *PaymentService) ensurePaymentSubscriptionReset(ctx context.Context, order *dbent.PaymentOrder, groupID int64, days int) error {
	target, err := subscriptionResetTarget(order)
	if err != nil {
		return err
	}
	if days < 1 || days > MaxValidityDays {
		return fmt.Errorf("invalid subscription reset term")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	lockedOrder, err := tx.PaymentOrder.Query().Where(paymentorder.IDEQ(order.ID)).ForUpdate().Only(txCtx)
	if err != nil {
		return err
	}
	applied, err := hasPaymentSubscriptionAssignmentAudit(txCtx, tx.Client(), order.ID)
	if err != nil {
		return err
	}
	if !applied {
		if lockedOrder.PaidAt == nil || (lockedOrder.Status != OrderStatusPaid && lockedOrder.Status != OrderStatusRecharging && lockedOrder.Status != OrderStatusFailed) {
			return infraerrors.BadRequest("INVALID_STATUS", "subscription reset requires a confirmed paid order")
		}
		sub, err := s.subscriptionSvc.userSubRepo.GetByIDForUpdate(txCtx, target)
		if err != nil {
			return err
		}
		if err := resetSubscriptionTerm(sub, order.UserID, groupID, days, time.Now()); err != nil {
			return err
		}
		if err := s.subscriptionSvc.userSubRepo.Update(txCtx, sub); err != nil {
			return err
		}
		detail, _ := json.Marshal(map[string]any{"action": "reset", "subscriptionID": target, "validityDays": days, "expiresAt": sub.ExpiresAt})
		if _, err := tx.PaymentAuditLog.Create().SetOrderID(strconv.FormatInt(order.ID, 10)).
			SetAction("SUBSCRIPTION_ASSIGNED").SetDetail(string(detail)).SetOperator("system").Save(txCtx); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.subscriptionSvc.invalidateSubscriptionCaches(order.UserID, groupID)
}

func resetSubscriptionTerm(sub *UserSubscription, userID, groupID int64, days int, now time.Time) error {
	if sub == nil || sub.UserID != userID || sub.GroupID != groupID || sub.DeletedAt != nil {
		return infraerrors.NotFound("SUBSCRIPTION_NOT_FOUND", "subscription not found")
	}
	if sub.Status != SubscriptionStatusActive && sub.Status != SubscriptionStatusExpired {
		return infraerrors.BadRequest("SUBSCRIPTION_RESET_UNAVAILABLE", "subscription is suspended or revoked")
	}
	if days < 1 || days > MaxValidityDays {
		return fmt.Errorf("invalid subscription reset term")
	}
	expiresAt := now.AddDate(0, 0, days)
	if expiresAt.After(MaxExpiresAt) {
		return fmt.Errorf("subscription reset expiry exceeds the maximum")
	}
	*sub = *renewedSubscriptionTerm(sub, "", now, expiresAt)
	return nil
}
