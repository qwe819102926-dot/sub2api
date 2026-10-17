package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPaidSubscriptionResetWeChatResumeKeepsSignedTarget(t *testing.T) {
	claims := &service.WeChatPaymentResumeClaims{OpenID: "test-openid", PaymentType: "wxpay", OrderType: "subscription", PlanID: 7, SubscriptionResetID: 5}
	request := &CreateOrderRequest{}
	require.NoError(t, applyWeChatPaymentResumeClaims(request, claims))
	require.Equal(t, int64(5), request.SubscriptionResetID)
	require.Equal(t, int64(7), request.PlanID)
	require.Error(t, applyWeChatPaymentResumeClaims(&CreateOrderRequest{SubscriptionResetID: 9}, claims))
	require.Error(t, applyWeChatPaymentResumeClaims(&CreateOrderRequest{SubscriptionResetID: 5}, &service.WeChatPaymentResumeClaims{OpenID: "test-openid", OrderType: "subscription", PlanID: 7}))
	require.Error(t, applyWeChatPaymentResumeClaims(nil, claims))
}
