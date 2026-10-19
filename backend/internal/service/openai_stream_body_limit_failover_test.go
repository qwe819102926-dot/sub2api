package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIRequestBodyLimitClassification(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"http_413", 413, `<html>413 Payload Too Large</html>`, true},
		{"http_400", 400, `{"error":{"type":"invalid_request_error","message":"Request payload is too large"}}`, true},
		{"failed_event", 400, `{"type":"response.failed","response":{"error":{"code":"invalid_request","message":"Request payload is too large"}}}`, true},
		{"explicit_code", 400, `{"error":{"code":"request_body_too_large","message":"size limit"}}`, true},
		{"explicit_status", 400, `{"response":{"error":{"status_code":413,"message":"size limit"}}}`, true},
		{"unknown_parameter", 400, `{"error":{"code":"unknown_parameter","message":"Unknown parameter: Request payload is too large"}}`, false},
		{"echoed_input", 400, `{"error":{"message":"Invalid input"},"input":"Request payload is too large"}`, false},
		{"context_413", 413, `{"error":{"code":"context_length_exceeded","message":"Request payload is too large"}}`, false},
		{"context_400", 400, `{"error":{"code":"context_length_exceeded","message":"maximum context length exceeded"}}`, false},
		{"unrelated_status", 401, `{"error":{"message":"Request payload is too large"}}`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isOpenAIRequestBodyTooLargeError(tt.status, "", []byte(tt.body)))
		})
	}
}

func TestOpenAIStreamBodyLimitFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		for _, terminal := range []string{
			`{"type":"response.failed","response":{"error":{"type":"invalid_request_error","code":"invalid_request","message":"Request payload is too large"}}}`,
			`{"type":"error","error":{"type":"invalid_request_error","message":"Request payload is too large"}}`,
		} {
			for _, outputStarted := range []bool{false, true} {
				t.Run(fmt.Sprintf("passthrough=%v/output=%v/%s", passthrough, outputStarted, terminal), func(t *testing.T) {
					stream := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_body_limit\"}}\n\n"
					// Observed on sub-coco: a whitespace heartbeat precedes the
					// request-body error and must not commit this upstream attempt.
					stream += openAIWhitespaceHeartbeatStream()
					if outputStarted {
						stream += "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial answer\"}\n\n"
					}
					stream += "data: " + terminal + "\n\n"
					rec := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(rec)
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
					resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(stream))}
					svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
					account := &Account{ID: 161, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"pool_mode": true}}
					var err error
					if passthrough {
						_, err = svc.handleStreamingResponsePassthrough(context.Background(), resp, c, account, time.Now(), "model", "model")
					} else {
						_, err = svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "model", "model")
					}
					require.Error(t, err)
					var failoverErr *UpstreamFailoverError
					if outputStarted {
						require.False(t, errors.As(err, &failoverErr), "partial answers must never be replayed on another account")
						require.Contains(t, rec.Body.String(), "partial answer")
						return
					}
					require.ErrorAs(t, err, &failoverErr)
					require.True(t, failoverErr.IsOpenAIRequestBodyTooLarge())
					require.Equal(t, http.StatusRequestEntityTooLarge, failoverErr.ClientStatusCode)
					require.False(t, failoverErr.RetryableOnSameAccount)
					require.True(t, failoverErr.ShouldRetryNextAccount())
					require.False(t, c.Writer.Written())
					require.Empty(t, rec.Body.String())
				})
			}
		}
	}
}

func TestOpenAINonStreamingSSEBodyLimitFailsOver(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		t.Run(fmt.Sprint(passthrough), func(t *testing.T) {
			c, rec := newNonStreamingFailoverContext(t)
			svc := newNonStreamingFailoverService()
			body := sseTerminalBody("response.failed", `{"type":"response.failed","response":{"error":{"type":"invalid_request_error","message":"Request payload is too large"}}}`)
			var err error
			if passthrough {
				_, err = svc.handlePassthroughSSEToJSON(newNonStreamingSSEResponse(), c, newNonStreamingFailoverAccount(), body, "model", "model")
			} else {
				_, err = svc.handleSSEToJSON(newNonStreamingSSEResponse(), c, newNonStreamingFailoverAccount(), body, "model", "model")
			}
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.True(t, failoverErr.IsOpenAIRequestBodyTooLarge())
			require.False(t, c.Writer.Written())
			require.Empty(t, rec.Body.String())
		})
	}
}

func TestOpenAIWhitespaceTextDoesNotCommitStream(t *testing.T) {
	for _, data := range []string{
		`{"type":"response.output_text.delta","delta":" \n\t"}`,
		`{"type":"response.output_text.done","text":" "}`,
		`{"type":"response.content_part.done","part":{"type":"output_text","text":" "}}`,
		`{"type":"response.output_item.done","item":{"type":"message","content":[{"type":"output_text","text":" "}]}}`,
		`{"type":"response.reasoning_summary_text.delta","delta":" "}`,
		`{"type":"response.content_part.added","part":{"type":"output_text","text":" "}}`,
		`{"type":"response.output_item.added","item":{"type":"message","content":[{"type":"output_text","text":" "}]}}`,
	} {
		eventType := gjson.Get(data, "type").String()
		require.False(t, openAIStreamDataStartsClientOutput(data, eventType), data)
		require.False(t, openAIStreamDataStartsVisibleOutput(data, eventType), data)
	}
	toolDelta := `{"type":"response.function_call_arguments.delta","delta":" "}`
	require.True(t, openAIStreamDataStartsClientOutput(toolDelta, "response.function_call_arguments.delta"), "tool argument bytes must remain authoritative")
	require.True(t, openAIStreamDataStartsVisibleOutput(toolDelta, "response.function_call_arguments.delta"))
}

func openAIWhitespaceHeartbeatStream() string {
	return strings.Join([]string{
		`data: {"type":"response.output_item.added","item":{"type":"message","content":[]}}`, "",
		`data: {"type":"response.content_part.added","part":{"type":"output_text","text":""}}`, "",
		`data: {"type":"response.output_text.delta","delta":" "}`, "",
		`data: {"type":"response.output_text.done","text":" "}`, "",
		`data: {"type":"response.content_part.done","part":{"type":"output_text","text":" "}}`, "",
		`data: {"type":"response.output_item.done","item":{"type":"message","content":[{"type":"output_text","text":" "}]}}`, "", "",
	}, "\n")
}

func TestOpenAIWhitespaceTextPreservedOnSuccessfulStream(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, text := range []string{"OK", ""} {
			t.Run(fmt.Sprintf("passthrough=%v/text=%q", passthrough, text), func(t *testing.T) {
				stream := openAIWhitespaceHeartbeatStream()
				if text != "" {
					stream += `data: {"type":"response.output_text.delta","delta":"OK"}` + "\n\n"
				}
				stream += `data: {"type":"response.completed","response":{"id":"resp_ok","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":" ` + text + `"}]}],"usage":{"input_tokens":1,"output_tokens":1}}}` + "\n\n"
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(stream))}
				svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
				account := &Account{ID: 161, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
				var err error
				if passthrough {
					_, err = svc.handleStreamingResponsePassthrough(context.Background(), resp, c, account, time.Now(), "model", "model")
				} else {
					_, err = svc.handleStreamingResponse(context.Background(), resp, c, account, time.Now(), "model", "model")
				}
				require.NoError(t, err)
				body := rec.Body.String()
				require.Equal(t, 1, strings.Count(body, `"delta":" "`), "buffered whitespace must be sent exactly once on success")
				if text != "" {
					require.Less(t, strings.Index(body, `"delta":" "`), strings.Index(body, `"delta":"OK"`))
				}
				require.Contains(t, body, "response.completed")
			})
		}
	}
}
