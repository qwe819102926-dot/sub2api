package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

func newTestJianPay(t *testing.T, baseURL string) *JianPay {
	t.Helper()
	provider, err := NewJianPay("test", map[string]string{"clientNo": "JP_TEST", "merchantKey": "secret", "apiBase": baseURL})
	if err != nil {
		t.Fatalf("NewJianPay: %v", err)
	}
	return provider
}

func TestJianPaySignExcludesSignatureFieldsAndSerializesObjects(t *testing.T) {
	params := map[string]any{
		"z": "last", "a": "first", "empty": "", "sign": "ignored", "sign_type": "MD5",
		"methodExpand": map[string]any{"b": "2", "a": "1"},
	}
	if got, want := jianPaySign(params, "key"), "78f0cdd09f00da9b3529512f29986fc8"; got != want {
		t.Fatalf("jianPaySign() = %q, want %q", got, want)
	}
}

func TestJianPayCreateAndQueryPayment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got := payload["sign"]; got != jianPaySign(payload, "secret") {
			t.Fatalf("invalid sign: %v", got)
		}
		switch r.URL.Path {
		case "/open/payment/pay/create":
			if payload["amount"] != float64(1234) || payload["payMethod"] != "wx" {
				t.Fatalf("unexpected create payload: %#v", payload)
			}
			_, _ = w.Write([]byte(`{"code":1000,"data":{"orderId":"JP_ORDER","payUrl":"https://checkout.example/pay"}}`))
		case "/open/payment/pay/info":
			_, _ = w.Write([]byte(`{"code":1000,"data":{"orderId":"JP_ORDER","amount":1234,"status":2,"paidAt":"2026-01-02 03:04:05"}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider := newTestJianPay(t, server.URL)
	created, err := provider.CreatePayment(context.Background(), payment.CreatePaymentRequest{OrderID: "merchant-order", Amount: "12.34", PaymentType: payment.TypeWxpay, Subject: "Test order", NotifyURL: "https://merchant.example/notify"})
	if err != nil {
		t.Fatalf("CreatePayment: %v", err)
	}
	if created.TradeNo != "JP_ORDER" || created.PayURL != "https://checkout.example/pay" || created.QRCode != created.PayURL {
		t.Fatalf("unexpected create response: %+v", created)
	}
	queried, err := provider.QueryOrder(context.Background(), created.TradeNo)
	if err != nil {
		t.Fatalf("QueryOrder: %v", err)
	}
	if queried.Status != payment.ProviderStatusPaid || queried.Amount != 12.34 || queried.TradeNo != "JP_ORDER" {
		t.Fatalf("unexpected query response: %+v", queried)
	}
}

func TestJianPayVerifyNotificationAndRejectRefund(t *testing.T) {
	provider := newTestJianPay(t, "https://jpay.example")
	payload := map[string]any{"clientNo": "JP_TEST", "orderId": "JP_ORDER", "merchantOrderNo": "merchant-order", "amount": 1234, "status": 2, "sign_type": "MD5"}
	payload["sign"] = jianPaySign(payload, "secret")
	raw, _ := json.Marshal(payload)
	notification, err := provider.VerifyNotification(context.Background(), string(raw), nil)
	if err != nil {
		t.Fatalf("VerifyNotification: %v", err)
	}
	if notification.Status != payment.ProviderStatusSuccess || notification.Amount != 12.34 || notification.OrderID != "merchant-order" {
		t.Fatalf("unexpected notification: %+v", notification)
	}
	payload["amount"] = 1
	tampered, _ := json.Marshal(payload)
	if _, err := provider.VerifyNotification(context.Background(), string(tampered), nil); err == nil {
		t.Fatal("VerifyNotification accepted a tampered payload")
	}
	if _, err := provider.Refund(context.Background(), payment.RefundRequest{}); err == nil {
		t.Fatal("Refund unexpectedly succeeded")
	}
}
