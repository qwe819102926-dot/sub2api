package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type upstreamRecoveryTestRepo struct {
	AccountRepository
	holds []UpstreamRecoveryState
}

func (r *upstreamRecoveryTestRepo) BeginUpstreamRecovery(_ context.Context, _ int64, state UpstreamRecoveryState) error {
	r.holds = append(r.holds, state)
	return nil
}

func (r *upstreamRecoveryTestRepo) ClaimUpstreamRecovery(context.Context, int, time.Time, time.Time) ([]UpstreamRecoveryClaim, error) {
	return nil, nil
}

func (r *upstreamRecoveryTestRepo) FinishUpstreamRecovery(context.Context, UpstreamRecoveryClaim, time.Time) error {
	return nil
}

func TestUpstreamRecoveryFailsOverForOptedInAccount(t *testing.T) {
	for _, tt := range []struct {
		name        string
		status      int
		contentType string
		body        string
		err         error
	}{
		{name: "not found", status: 404, body: "route unavailable"},
		{name: "authentication failure", status: 401, body: "unauthorized"},
		{name: "transport failure", status: 502, err: errors.New("connection reset")},
		{name: "json failure", status: 200, body: `{"error":{"code":"upstream_failure","message":"try again"}}`},
		{name: "stream failure after metadata", status: 200, contentType: "text/event-stream",
			body: "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n" +
				"data: {\"error\":{\"message\":\"Unavailable\"}}\n\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{ID: 4, Platform: PlatformDeepseek, Type: AccountTypeAPIKey,
				Credentials: map[string]any{UpstreamRecoveryEnabledCredential: true}}
			repo := &upstreamRecoveryTestRepo{}
			upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{{
				StatusCode: tt.status, Header: http.Header{"Content-Type": {tt.contentType}},
				Body: io.NopCloser(strings.NewReader(tt.body)),
			}}}
			svc := &OpenAIGatewayService{accountRepo: repo, httpUpstream: upstream}
			if tt.err != nil {
				svc.httpUpstream = &httpUpstreamStub{err: tt.err}
			}
			ctx := context.Background()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://upstream.example/v1/chat/completions",
				strings.NewReader(`{"model":"mapped-model","stream":true,"protocol":"ignored"}`))
			require.NoError(t, err)

			resp, err := svc.doOpenAITextUpstream(ctx, nil, req, "", account)
			require.Nil(t, resp)
			var failover *UpstreamFailoverError
			require.ErrorAs(t, err, &failover)
			expected := tt.status
			if expected == 200 {
				expected = 502
			}
			require.Equal(t, expected, failover.StatusCode)
			require.False(t, failover.RetryableOnSameAccount)
			require.Len(t, repo.holds, 1)
			hold := repo.holds[0]
			require.Equal(t, "mapped-model", hold.Model)
			require.Equal(t, APIProtocolChatCompletions, hold.Protocol)
			require.True(t, hold.Stream)
			require.WithinDuration(t, time.Now().Add(2*time.Minute), time.UnixMilli(hold.NextProbeAtMS), time.Second)

			state := map[string]any{UpstreamRecoveryStateExtra: map[string]any{"next_probe_at_ms": 1}}
			account.Status, account.Schedulable, account.Extra = StatusActive, true, state
			require.False(t, account.IsSchedulable(), "probe deadline alone must not release the hold")
			delete(account.Credentials, UpstreamRecoveryEnabledCredential)
			require.True(t, account.IsSchedulable())
		})
	}
}

func TestUpstreamRecoveryLeavesAnOptedOutAccountOnLegacyPath(t *testing.T) {
	account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	repo := &upstreamRecoveryTestRepo{}
	upstream := &queuedHTTPUpstreamStub{responses: []*http.Response{{
		StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("original response")),
	}}}
	svc := &OpenAIGatewayService{httpUpstream: upstream, accountRepo: repo}
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://upstream.example/v1/chat/completions",
		strings.NewReader(`{"model":"mapped-model"}`))
	require.NoError(t, err)
	resp, err := svc.doOpenAITextUpstream(ctx, nil, req, "", account)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Empty(t, repo.holds)
	require.NoError(t, resp.Body.Close())
}

func TestUpstreamRecoveryStreamPreservesBytesAndHoldsOnLateError(t *testing.T) {
	stream := "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n" +
		"data: {\"error\":{\"message\":\"lost upstream\"}}\n\n"
	response := &http.Response{
		Header: http.Header{"Content-Type": {"text/event-stream"}},
		Body:   io.NopCloser(strings.NewReader(stream)),
	}
	holds := 0
	body, err := inspectUpstreamRecoveryResponse(response,
		UpstreamRecoveryState{Protocol: APIProtocolChatCompletions}, func() { holds++ })
	require.NoError(t, err)
	require.Zero(t, holds)
	got, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, stream, string(got))
	require.Equal(t, 1, holds)
	require.NoError(t, body.Close())
}

func TestUpstreamRecoveryProbeNeedsValidModelOutput(t *testing.T) {
	for _, tt := range []struct {
		payload string
		success bool
	}{
		{`{"choices":[{"message":{"role":"assistant","content":"OK"}}]}`, true},
		{`{"error":{"code":"upstream_failure","message":"try again"}}`, false},
		{`{"choices":[]}`, false},
		{"data: {\"choices\":[{\"delta\":{\"content\":\"OK\"}}]}\n\ndata: [DONE]\n\n", true},
		{"data: {\"choices\":[{\"delta\":{\"content\":\"OK\"}}]}\n\n", false},
	} {
		resp := &http.Response{Body: io.NopCloser(strings.NewReader(tt.payload))}
		err := validateUpstreamRecoveryProbe(resp, APIProtocolChatCompletions)
		require.Equal(t, tt.success, err == nil, "%s", tt.payload)
	}
}
