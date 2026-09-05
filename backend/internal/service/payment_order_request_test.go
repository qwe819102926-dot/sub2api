package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func TestBuildProviderCreatePaymentRequestIncludesConfiguredNotifyURL(t *testing.T) {
	selection := &payment.InstanceSelection{
		Config: map[string]string{"notifyUrl": " https://merchant.example/payment/notify "},
	}

	request := buildProviderCreatePaymentRequest(CreateOrderRequest{PaymentType: payment.TypeAlipay}, selection, "order-1", "12.34", "Test order")
	if request.NotifyURL != "https://merchant.example/payment/notify" {
		t.Fatalf("NotifyURL = %q", request.NotifyURL)
	}
}
