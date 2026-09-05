package provider

import (
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/shopspring/decimal"
)

const (
	jianPayDefaultAPIBase  = "https://jpay.hzjianban.com"
	jianPayHTTPTimeout     = 10 * time.Second
	maxJianPayResponseSize = 1 << 20
)

// JianPay implements JianPay's hosted checkout flow. Merchant credentials remain
// exclusively in the encrypted provider-instance configuration.
type JianPay struct {
	instanceID string
	config     map[string]string
	httpClient *http.Client
}

func NewJianPay(instanceID string, config map[string]string) (*JianPay, error) {
	for _, key := range []string{"clientNo", "merchantKey"} {
		if strings.TrimSpace(config[key]) == "" {
			return nil, fmt.Errorf("jianpay config missing required key: %s", key)
		}
	}
	cfg := cloneStringMap(config)
	base, err := normalizeJianPayAPIBase(cfg["apiBase"])
	if err != nil {
		return nil, err
	}
	cfg["apiBase"] = base
	return &JianPay{instanceID: instanceID, config: cfg, httpClient: &http.Client{Timeout: jianPayHTTPTimeout}}, nil
}

func normalizeJianPayAPIBase(raw string) (string, error) {
	base := strings.TrimSpace(raw)
	if base == "" {
		return jianPayDefaultAPIBase, nil
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return "", fmt.Errorf("jianpay config apiBase must be an absolute http/https URL")
	}
	parsed.RawQuery, parsed.Fragment = "", ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (j *JianPay) Name() string        { return "JianPay" }
func (j *JianPay) ProviderKey() string { return payment.TypeJianPay }
func (j *JianPay) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeAlipay, payment.TypeWxpay}
}
func (j *JianPay) MerchantIdentityMetadata() map[string]string {
	if j == nil || strings.TrimSpace(j.config["clientNo"]) == "" {
		return nil
	}
	return map[string]string{"client_no": strings.TrimSpace(j.config["clientNo"])}
}

func (j *JianPay) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	amount, err := jianPayAmountToCents(req.Amount)
	if err != nil {
		return nil, fmt.Errorf("jianpay amount: %w", err)
	}
	method, err := jianPayMethod(req.PaymentType)
	if err != nil {
		return nil, err
	}
	params := map[string]any{
		"clientNo": j.config["clientNo"], "amount": amount, "orderNo": req.OrderID,
		"goodsName": req.Subject, "payMethod": method, "sign_type": "MD5",
	}
	if strings.TrimSpace(req.NotifyURL) != "" {
		params["notifyUrl"] = req.NotifyURL
	}
	if strings.TrimSpace(req.ReturnURL) != "" {
		params["returnUrl"] = req.ReturnURL
	}
	params["sign"] = jianPaySign(params, j.config["merchantKey"])
	body, err := j.post(ctx, "/open/payment/pay/create", params)
	if err != nil {
		return nil, fmt.Errorf("jianpay create: %w", err)
	}
	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			OrderID string `json:"orderId"`
			PayURL  string `json:"payUrl"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("jianpay parse create response: %w", err)
	}
	if response.Code != 1000 {
		return nil, fmt.Errorf("jianpay create failed: %s", response.Message)
	}
	if strings.TrimSpace(response.Data.OrderID) == "" || strings.TrimSpace(response.Data.PayURL) == "" {
		return nil, fmt.Errorf("jianpay create response missing orderId or payUrl")
	}
	// The existing frontend renders QRCode itself, so pass the checkout URL rather
	// than JianPay's QR image URL to keep it directly scannable.
	return &payment.CreatePaymentResponse{TradeNo: response.Data.OrderID, PayURL: response.Data.PayURL, QRCode: response.Data.PayURL}, nil
}

// CreateAPIPayment exposes JianPay's API-native order endpoint for callers that
// need native QR/action URLs or JSAPI parameters. The regular payment flow uses
// the hosted checkout above.
func (j *JianPay) CreateAPIPayment(ctx context.Context, req payment.CreatePaymentRequest, payType string, methodExpand map[string]any) (*payment.CreatePaymentResponse, error) {
	amount, err := jianPayAmountToCents(req.Amount)
	if err != nil {
		return nil, fmt.Errorf("jianpay amount: %w", err)
	}
	method, err := jianPayMethod(req.PaymentType)
	if err != nil {
		return nil, err
	}
	params := map[string]any{"clientNo": j.config["clientNo"], "amount": amount, "orderNo": req.OrderID, "goodsName": req.Subject, "payMethod": method, "payType": payType, "sign_type": "MD5"}
	if len(methodExpand) > 0 {
		params["methodExpand"] = methodExpand
	}
	if req.NotifyURL != "" {
		params["notifyUrl"] = req.NotifyURL
	}
	if req.ReturnURL != "" {
		params["returnUrl"] = req.ReturnURL
	}
	params["sign"] = jianPaySign(params, j.config["merchantKey"])
	body, err := j.post(ctx, "/open/payment/pay/api-create", params)
	if err != nil {
		return nil, fmt.Errorf("jianpay api create: %w", err)
	}
	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			OrderID      string                      `json:"orderId"`
			QRCode       string                      `json:"qrCode"`
			PayActionURL string                      `json:"payActionUrl"`
			PayInfo      *payment.WechatJSAPIPayload `json:"payInfo"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("jianpay parse api create response: %w", err)
	}
	if response.Code != 1000 {
		return nil, fmt.Errorf("jianpay api create failed: %s", response.Message)
	}
	return &payment.CreatePaymentResponse{TradeNo: response.Data.OrderID, QRCode: response.Data.QRCode, PayURL: response.Data.PayActionURL, JSAPI: response.Data.PayInfo}, nil
}

func jianPayMethod(paymentType string) (string, error) {
	switch payment.GetBasePaymentType(strings.TrimSpace(paymentType)) {
	case payment.TypeAlipay:
		return "alipay", nil
	case payment.TypeWxpay:
		return "wx", nil
	default:
		return "", fmt.Errorf("jianpay does not support payment type %q", paymentType)
	}
}

func (j *JianPay) QueryOrder(ctx context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	if strings.TrimSpace(tradeNo) == "" {
		return nil, fmt.Errorf("jianpay query requires orderId")
	}
	params := map[string]any{"clientNo": j.config["clientNo"], "orderId": tradeNo, "sign_type": "MD5"}
	params["sign"] = jianPaySign(params, j.config["merchantKey"])
	body, err := j.post(ctx, "/open/payment/pay/info", params)
	if err != nil {
		return nil, fmt.Errorf("jianpay query: %w", err)
	}
	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			OrderID string          `json:"orderId"`
			Amount  json.RawMessage `json:"amount"`
			Status  int             `json:"status"`
			PaidAt  string          `json:"paidAt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("jianpay parse query response: %w", err)
	}
	if response.Code != 1000 {
		return nil, fmt.Errorf("jianpay query failed: %s", response.Message)
	}
	amount, err := jianPayCentsToAmount(response.Data.Amount)
	if err != nil {
		return nil, fmt.Errorf("jianpay query amount: %w", err)
	}
	status := payment.ProviderStatusPending
	switch response.Data.Status {
	case 2:
		status = payment.ProviderStatusPaid
	case 3, 4:
		status = payment.ProviderStatusFailed
	}
	if strings.TrimSpace(response.Data.OrderID) == "" {
		response.Data.OrderID = tradeNo
	}
	return &payment.QueryOrderResponse{TradeNo: response.Data.OrderID, Status: status, Amount: amount, PaidAt: response.Data.PaidAt, Metadata: j.MerchantIdentityMetadata()}, nil
}

func (j *JianPay) VerifyNotification(_ context.Context, rawBody string, _ map[string]string) (*payment.PaymentNotification, error) {
	var payload map[string]any
	decoder := json.NewDecoder(strings.NewReader(rawBody))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("jianpay parse notification: %w", err)
	}
	receivedSign, _ := payload["sign"].(string)
	if receivedSign == "" || subtle.ConstantTimeCompare([]byte(strings.ToLower(receivedSign)), []byte(jianPaySign(payload, j.config["merchantKey"]))) != 1 {
		return nil, fmt.Errorf("jianpay notification signature mismatch")
	}
	clientNo, _ := payload["clientNo"].(string)
	if strings.TrimSpace(clientNo) != strings.TrimSpace(j.config["clientNo"]) {
		return nil, fmt.Errorf("jianpay notification clientNo mismatch")
	}
	tradeNo, _ := payload["orderId"].(string)
	orderID, _ := payload["merchantOrderNo"].(string)
	if strings.TrimSpace(tradeNo) == "" || strings.TrimSpace(orderID) == "" {
		return nil, fmt.Errorf("jianpay notification missing order id")
	}
	amount, err := jianPayCentsToAmountValue(payload["amount"])
	if err != nil {
		return nil, fmt.Errorf("jianpay notification amount: %w", err)
	}
	status := payment.ProviderStatusFailed
	if code, err := jianPayStatus(payload["status"]); err == nil && code == 2 {
		status = payment.ProviderStatusSuccess
	}
	metadata := j.MerchantIdentityMetadata()
	return &payment.PaymentNotification{TradeNo: tradeNo, OrderID: orderID, Amount: amount, Status: status, RawData: rawBody, Metadata: metadata}, nil
}

// Refund is intentionally disabled for this integration per merchant policy.
func (j *JianPay) Refund(context.Context, payment.RefundRequest) (*payment.RefundResponse, error) {
	return nil, fmt.Errorf("jianpay refunds are not enabled")
}

func (j *JianPay) post(ctx context.Context, path string, payload map[string]any) ([]byte, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, j.config["apiBase"]+path, strings.NewReader(string(encoded)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := j.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxJianPayResponseSize))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected HTTP status %d: %s", resp.StatusCode, truncateJianPayError(string(body)))
	}
	return body, nil
}

func jianPaySign(params map[string]any, merchantKey string) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		if key != "sign" && key != "sign_type" && !jianPayEmpty(value) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, key+"="+jianPayString(params[key]))
	}
	sum := md5.Sum([]byte(strings.Join(pairs, "&") + merchantKey))
	return hex.EncodeToString(sum[:])
}

func jianPayEmpty(value any) bool {
	return value == nil || (func() bool { s, ok := value.(string); return ok && s == "" })()
}
func jianPayString(value any) string {
	if object, ok := value.(map[string]any); ok {
		encoded, _ := json.Marshal(object)
		return string(encoded)
	}
	return fmt.Sprint(value)
}
func jianPayAmountToCents(raw string) (int64, error) {
	amount, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil || amount.IsNegative() || !amount.Mul(decimal.NewFromInt(100)).Equal(amount.Mul(decimal.NewFromInt(100)).Truncate(0)) {
		return 0, fmt.Errorf("must be a non-negative amount with at most two decimal places")
	}
	return amount.Mul(decimal.NewFromInt(100)).IntPart(), nil
}
func jianPayCentsToAmount(raw json.RawMessage) (float64, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, err
	}
	return jianPayCentsToAmountValue(value)
}
func jianPayCentsToAmountValue(value any) (float64, error) {
	cents, err := decimal.NewFromString(strings.TrimSpace(fmt.Sprint(value)))
	if err != nil {
		return 0, err
	}
	result, _ := cents.Div(decimal.NewFromInt(100)).Float64()
	return result, nil
}
func jianPayStatus(value any) (int, error) { return strconv.Atoi(strings.TrimSpace(fmt.Sprint(value))) }
func truncateJianPayError(value string) string {
	if len(value) > 512 {
		return value[:512]
	}
	return value
}

var (
	_ payment.Provider                 = (*JianPay)(nil)
	_ payment.MerchantIdentityProvider = (*JianPay)(nil)
)
