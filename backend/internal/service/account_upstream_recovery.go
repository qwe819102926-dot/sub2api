package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	UpstreamRecoveryEnabledCredential = "upstream_failure_recovery_enabled"
	UpstreamRecoveryStateExtra        = "upstream_failure_recovery"
	upstreamRecoveryCooldown          = 2 * time.Minute
	upstreamRecoveryProbeTimeout      = 25 * time.Second
	upstreamRecoveryLease             = 40 * time.Second
	upstreamRecoveryPollInterval      = 5 * time.Second
	upstreamRecoveryBodyLimit         = 64 << 10
)

type UpstreamRecoveryState struct {
	Protocol      string `json:"protocol"`
	Model         string `json:"model"`
	Stream        bool   `json:"stream"`
	NextProbeAtMS int64  `json:"next_probe_at_ms"`
	LeaseUntilMS  int64  `json:"lease_until_ms"`
}

type UpstreamRecoveryClaim struct {
	AccountID int64
	State     UpstreamRecoveryState
}

// UpstreamRecoveryRepository stores the hold independently of other account limits.
// Claims are leased so a restart or a second replica cannot release a hold early.
type UpstreamRecoveryRepository interface {
	BeginUpstreamRecovery(context.Context, int64, UpstreamRecoveryState) error
	ClaimUpstreamRecovery(context.Context, int, time.Time, time.Time) ([]UpstreamRecoveryClaim, error)
	FinishUpstreamRecovery(context.Context, UpstreamRecoveryClaim, time.Time) error
}

func (a *Account) UpstreamRecoveryEnabled() bool {
	if a == nil || a.Type != AccountTypeAPIKey || (!a.IsOpenAI() && !a.IsCNProvider()) {
		return false
	}
	enabled, _ := a.Credentials[UpstreamRecoveryEnabledCredential].(bool)
	return enabled
}

func (a *Account) isAwaitingUpstreamRecovery() bool {
	return a.UpstreamRecoveryEnabled() && a.Extra[UpstreamRecoveryStateExtra] != nil
}

func upstreamRecoveryRequest(req *http.Request) (UpstreamRecoveryState, bool) {
	if req.Method != http.MethodPost || req.GetBody == nil {
		return UpstreamRecoveryState{}, false
	}
	state := UpstreamRecoveryState{}
	switch path := req.URL.Path; {
	case strings.HasSuffix(path, "/chat/completions"):
		state.Protocol = APIProtocolChatCompletions
	case strings.HasSuffix(path, "/responses"):
		state.Protocol = APIProtocolResponses
	case strings.HasSuffix(path, "/responses/compact"):
		state.Protocol = AccountTestModeCompact
	case strings.HasSuffix(path, "/messages"):
		state.Protocol = APIProtocolAnthropic
	default:
		return state, false
	}
	body, err := req.GetBody()
	if err != nil {
		return state, false
	}
	defer func() { _ = body.Close() }()
	var payload struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	err = json.NewDecoder(body).Decode(&payload)
	state.Model = strings.TrimSpace(payload.Model)
	state.Stream = payload.Stream
	return state, err == nil && state.Model != ""
}

type upstreamRecoveryAttempt struct {
	accountID int64
	state     UpstreamRecoveryState
	once      sync.Once
}

const upstreamRecoveryAttemptKey = "openai_upstream_recovery_attempt"

func (s *OpenAIGatewayService) holdUpstreamRecoveryAttempt(ctx context.Context, c *gin.Context, account *Account, status int) bool {
	if c == nil || !account.UpstreamRecoveryEnabled() {
		return false
	}
	value, _ := c.Get(upstreamRecoveryAttemptKey)
	attempt, ok := value.(*upstreamRecoveryAttempt)
	if !ok || attempt.accountID != account.ID {
		return false
	}
	attempt.hold(ctx, s, account, status)
	return true
}

func (a *upstreamRecoveryAttempt) hold(ctx context.Context, s *OpenAIGatewayService, account *Account, status int) {
	if ctx.Err() == nil {
		a.once.Do(func() { s.holdAccountForUpstreamRecovery(ctx, account, a.state, status) })
	}
}

// doOpenAITextUpstream applies the opt-in rule at the actual HTTP attempt,
// before protocol handlers can retry the same account or commit a response.
func (s *OpenAIGatewayService) doOpenAITextUpstream(
	ctx context.Context, c *gin.Context, req *http.Request, proxyURL string, account *Account,
) (*http.Response, error) {
	if !account.UpstreamRecoveryEnabled() {
		return s.doOpenAIUpstream(req, proxyURL, account)
	}
	state, ok := upstreamRecoveryRequest(req)
	if !ok {
		return s.doOpenAIUpstream(req, proxyURL, account)
	}
	attempt := &upstreamRecoveryAttempt{accountID: account.ID, state: state}
	if c != nil {
		c.Set(upstreamRecoveryAttemptKey, attempt)
	}
	attemptCtx, cancel := context.WithCancel(req.Context())
	stopClientCancel := context.AfterFunc(ctx, cancel)
	defer stopClientCancel()
	keepOpen := false
	defer func() {
		if !keepOpen {
			cancel()
		}
	}()
	resp, err := s.doOpenAIUpstream(req.WithContext(attemptCtx), proxyURL, account)
	if ctx.Err() != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil, ctx.Err()
	}
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		attempt.hold(ctx, s, account, 0)
		return nil, upstreamRecoveryFailover(c, account, nil, http.StatusBadGateway, openAITransportFailoverBody)
	}

	// Keep the configured stream timeout during the preflight read. The
	// protocol handler owns subsequent reads and their timeouts.
	var readTimer *time.Timer
	if s.cfg != nil && s.cfg.Gateway.StreamDataIntervalTimeout > 0 {
		readTimer = time.AfterFunc(time.Duration(s.cfg.Gateway.StreamDataIntervalTimeout)*time.Second, cancel)
		defer readTimer.Stop()
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body := s.readUpstreamErrorBody(resp)
		_ = resp.Body.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		attempt.hold(ctx, s, account, resp.StatusCode)
		return nil, upstreamRecoveryFailover(c, account, resp.Header, resp.StatusCode, body)
	}

	resp.Body, err = inspectUpstreamRecoveryResponse(resp, state, func() {
		attempt.hold(ctx, s, account, http.StatusBadGateway)
	})
	if err != nil {
		_ = resp.Body.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		attempt.hold(ctx, s, account, http.StatusBadGateway)
		return nil, upstreamRecoveryFailover(c, account, resp.Header, http.StatusBadGateway, openAITransportFailoverBody)
	}
	if (readTimer != nil && !readTimer.Stop()) || ctx.Err() != nil {
		_ = resp.Body.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		attempt.hold(ctx, s, account, http.StatusBadGateway)
		return nil, upstreamRecoveryFailover(c, account, resp.Header, http.StatusBadGateway, openAITransportFailoverBody)
	}
	resp.Body = &openAIRequestContextReadCloser{ReadCloser: resp.Body, cleanup: cancel}
	keepOpen = true
	return resp, nil
}

func (s *OpenAIGatewayService) holdAccountForUpstreamRecovery(ctx context.Context, account *Account, state UpstreamRecoveryState, status int) {
	if ctx.Err() != nil {
		return
	}
	repo, ok := s.accountRepo.(UpstreamRecoveryRepository)
	if !ok {
		return
	}
	state.NextProbeAtMS = time.Now().Add(upstreamRecoveryCooldown).UnixMilli()
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIAccountStateUpdateTimeout)
	defer cancel()
	if err := repo.BeginUpstreamRecovery(persistCtx, account.ID, state); err != nil {
		slog.Error("upstream_recovery_hold_failed", "account_id", account.ID, "status", status, "error", err)
	}
}

func upstreamRecoveryFailover(c *gin.Context, account *Account, headers http.Header, status int, body []byte) error {
	message := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(body))
	setOpsUpstreamError(c, status, message, "")
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		AccountID:          account.ID,
		AccountName:        account.Name,
		Platform:           account.Platform,
		ProxyID:            opsUpstreamProxyID(account),
		ProxyName:          opsUpstreamProxyName(account),
		UpstreamStatusCode: status,
		UpstreamRequestID:  headers.Get("x-request-id"),
		Kind:               "failover",
		Message:            message,
	})
	return &UpstreamFailoverError{StatusCode: status, ResponseBody: body, ResponseHeaders: headers}
}

type recoveryReadCloser struct {
	io.Reader
	io.Closer
}

// Keep the response bytes intact for the existing protocol parser. Inspect only
// a bounded prelude before giving it the stream; later failures still hold the account.
func inspectUpstreamRecoveryResponse(resp *http.Response, state UpstreamRecoveryState, onFailure func()) (io.ReadCloser, error) {
	body := resp.Body
	reader := bufio.NewReaderSize(body, upstreamRecoveryBodyLimit)
	sse, err := upstreamRecoveryIsSSE(reader, resp.Header)
	if err != nil {
		return body, err
	}
	// Some compatibility gateways return an HTTP 200 JSON error to a streaming
	// request. A leading object takes precedence over the SSE content type.
	if !sse {
		payload, readErr := io.ReadAll(io.LimitReader(reader, upstreamRecoveryBodyLimit+1))
		if readErr != nil {
			return body, readErr
		}
		if len(payload) <= upstreamRecoveryBodyLimit &&
			(!gjson.ValidBytes(payload) || !upstreamRecoveryValidJSON(payload, state.Protocol)) {
			return body, fmt.Errorf("upstream returned an error or invalid JSON")
		}
		return &recoveryReadCloser{Reader: io.MultiReader(bytes.NewReader(payload), reader), Closer: body}, nil
	}

	stream := &upstreamRecoveryStream{reader: reader, protocol: state.Protocol}
	var prelude bytes.Buffer
	for prelude.Len() < upstreamRecoveryBodyLimit {
		line, readErr := stream.next()
		if stream.failed {
			return body, fmt.Errorf("upstream stream failed before output")
		}
		prelude.Write(line)
		if stream.output {
			break
		}
		if stream.terminal && stream.protocol != APIProtocolChatCompletions {
			return body, fmt.Errorf("upstream stream completed without a model response")
		}
		if readErr != nil {
			return body, readErr
		}
	}
	stream.onFailure = onFailure
	return &recoveryReadCloser{Reader: io.MultiReader(bytes.NewReader(prelude.Bytes()), stream), Closer: body}, nil
}

func upstreamRecoveryValidJSON(body []byte, protocol string) bool {
	frame := gjson.ParseBytes(body)
	return !upstreamRecoveryFrameFailed(frame) && upstreamRecoveryFrameOutput(frame, protocol)
}

func upstreamRecoveryIsSSE(reader *bufio.Reader, headers http.Header) (bool, error) {
	for size := 1; size <= 64; size++ {
		prefix, err := reader.Peek(size)
		if trimmed := bytes.TrimSpace(prefix); len(trimmed) > 0 {
			first := trimmed[0]
			return first != '{' && (isEventStreamResponse(headers) || first == 'd' || first == 'e' || first == ':'), nil
		}
		if err != nil {
			return false, err
		}
	}
	return isEventStreamResponse(headers), nil
}

type upstreamRecoveryStream struct {
	reader        *bufio.Reader
	onFailure     func()
	protocol      string
	pending       []byte
	readErr       error
	lineContinues bool
	failed        bool
	output        bool
	terminal      bool
}

func (s *upstreamRecoveryStream) next() ([]byte, error) {
	line, err := s.reader.ReadSlice('\n')
	if !s.lineContinues && err != bufio.ErrBufferFull {
		text := strings.TrimSpace(string(line))
		if text == "event: error" {
			s.fail()
		}
		if data, ok := extractOpenAISSEDataLine(text); ok {
			payload := strings.TrimSpace(data)
			if payload == "[DONE]" {
				s.terminal = true
			} else {
				frame := gjson.Parse(payload)
				if upstreamRecoveryFrameFailed(frame) {
					s.fail()
				}
				if upstreamRecoveryFrameOutput(frame, s.protocol) {
					s.output = true
				}
				if upstreamRecoveryFrameTerminal(frame, s.protocol) {
					s.terminal = true
				}
			}
		}
	}
	s.lineContinues = err == bufio.ErrBufferFull
	if s.lineContinues {
		err = nil
	}
	if err != nil && !s.terminal {
		s.fail()
	}
	return line, err
}

func (s *upstreamRecoveryStream) fail() {
	if !s.failed {
		s.failed = true
		if s.onFailure != nil {
			s.onFailure()
		}
	}
}

func (s *upstreamRecoveryStream) Read(p []byte) (int, error) {
	if len(s.pending) == 0 && s.readErr == nil {
		s.pending, s.readErr = s.next()
	}
	n := copy(p, s.pending)
	s.pending = s.pending[n:]
	if len(s.pending) == 0 {
		return n, s.readErr
	}
	return n, nil
}

func upstreamRecoveryFrameFailed(frame gjson.Result) bool {
	if upstreamRecoveryHasError(frame.Get("error")) || upstreamRecoveryHasError(frame.Get("response.error")) {
		return true
	}
	switch frame.Get("type").String() {
	case "error", "response.failed", "response.error":
		return true
	default:
		return frame.Get("status").String() == "failed" || frame.Get("response.status").String() == "failed"
	}
}

func upstreamRecoveryHasError(value gjson.Result) bool {
	return value.Exists() && value.Type != gjson.Null && value.Type != gjson.False &&
		value.Raw != `""` && !(value.IsObject() && len(value.Map()) == 0)
}

func upstreamRecoveryFrameOutput(frame gjson.Result, protocol string) bool {
	switch protocol {
	case APIProtocolChatCompletions:
		for _, choice := range frame.Get("choices").Array() {
			delta := choice.Get("delta")
			if choice.Get("message").IsObject() || delta.Get("content").String() != "" ||
				delta.Get("reasoning_content").String() != "" || delta.Get("reasoning").String() != "" ||
				delta.Get("tool_calls").IsArray() || delta.Get("function_call").IsObject() {
				return true
			}
		}
	case APIProtocolAnthropic:
		return frame.Get("type").String() == "content_block_delta" || len(frame.Get("content").Array()) > 0
	default:
		return openAIStreamDataStartsVisibleOutput(frame.Raw, frame.Get("type").String()) ||
			len(frame.Get("output").Array()) > 0 || len(frame.Get("response.output").Array()) > 0
	}
	return false
}

func upstreamRecoveryFrameTerminal(frame gjson.Result, protocol string) bool {
	switch protocol {
	case APIProtocolChatCompletions:
		for _, choice := range frame.Get("choices").Array() {
			if choice.Get("finish_reason").String() != "" {
				return true
			}
		}
	case APIProtocolAnthropic:
		return frame.Get("type").String() == "message_stop"
	default:
		switch frame.Get("type").String() {
		case "response.completed", "response.done", "response.incomplete":
			return true
		}
	}
	return false
}
