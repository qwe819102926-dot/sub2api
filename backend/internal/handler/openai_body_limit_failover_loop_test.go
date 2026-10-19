//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type bodyLimitFailoverUpstream struct {
	service.HTTPUpstream
	hits        []int64
	bodies      [][]byte
	firstStatus int
}

func (u *bodyLimitFailoverUpstream) Do(req *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.hits = append(u.hits, accountID)
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	u.bodies = append(u.bodies, body)
	if accountID == 2152 {
		if u.firstStatus != http.StatusOK {
			return &http.Response{
				StatusCode: u.firstStatus,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"invalid_request_error","message":"Request payload is too large"}}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(
				"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_rejected\"}}\n\n" +
					"data: {\"type\":\"response.output_text.delta\",\"delta\":\" \"}\n\n" +
					"data: {\"type\":\"response.output_text.done\",\"text\":\" \"}\n\n" +
					"data: {\"type\":\"response.content_part.done\",\"part\":{\"type\":\"output_text\",\"text\":\" \"}}\n\n" +
					"data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\" \"}]}}\n\n" +
					"data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"type\":\"invalid_request_error\",\"message\":\"Request payload is too large\"}}}\n\n")),
		}, nil
	}
	// Use a deterministic terminal failure to exercise exhaustion as well as
	// routing. No billing dependencies or real model requests are needed here.
	return &http.Response{
		StatusCode: http.StatusRequestEntityTooLarge,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"larger upstream body limit exceeded"}}`)),
	}, nil
}

func bodyLimitTestAccount(id int64, priority int, baseURL string) service.Account {
	return service.Account{
		ID: id, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Priority: priority,
		Credentials: map[string]any{"api_key": "test-key", "base_url": baseURL},
		Extra:       map[string]any{"openai_responses_supported": true},
	}
}

func TestResponsesBodyLimitFailoverSkipsSameUpstreamAndPreservesRequest(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusBadRequest, http.StatusRequestEntityTooLarge} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := &bodyLimitFailoverUpstream{firstStatus: status}
			h := newOpenAIResponsesFailoverTestHandler(t, upstream,
				bodyLimitTestAccount(2152, 0, "https://www.sub-coco.com"),
				bodyLimitTestAccount(2155, 1, "https://www.sub-coco.com/v1/"),
				bodyLimitTestAccount(2156, 2, "https://www.sub-coco.com/"),
				bodyLimitTestAccount(2163, 3, "https://api.cicadas.top/"),
			)
			// Skipping duplicate routes must not spend extra upstream-attempt budget.
			h.maxAccountSwitches = 1
			c, rec := newOpenAIResponsesFailoverTestContext(t, context.Background())
			h.Responses(c)

			require.Equal(t, []int64{2152, 2163}, upstream.hits)
			require.Len(t, upstream.bodies, 2)
			require.JSONEq(t, string(upstream.bodies[0]), string(upstream.bodies[1]), "failover must replay the same input")
			require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), service.OpenAIRequestBodyTooLargeClientMessage)
			require.NotContains(t, rec.Body.String(), "resp_rejected", "the rejected upstream's preamble must stay buffered")
		})
	}
}

func TestBodyLimitFailoverStateReleasesSkippedSlotAndIsRequestLocal(t *testing.T) {
	state := make(openAIRequestBodyFailoverState)
	first := bodyLimitTestAccount(2152, 0, "https://www.sub-coco.com")
	state.record(&first, bodyLimitFailoverTestError())
	sameRoute := bodyLimitTestAccount(2155, 1, "https://WWW.SUB-COCO.COM/v1/")
	releaseCalls := 0
	selection := &service.AccountSelectionResult{Account: &sameRoute, ReleaseFunc: func() { releaseCalls++ }}
	excluded := make(map[int64]struct{})
	require.True(t, state.skip(selection, excluded))
	require.Contains(t, excluded, sameRoute.ID)
	require.Equal(t, 1, releaseCalls)
	require.Nil(t, selection.ReleaseFunc)

	nextRequest := make(openAIRequestBodyFailoverState)
	require.False(t, nextRequest.skip(selection, make(map[int64]struct{})))
	otherRoute := bodyLimitTestAccount(2163, 3, "https://api.cicadas.top/")
	require.False(t, state.skip(&service.AccountSelectionResult{Account: &otherRoute}, excluded))
	state.record(&otherRoute, &service.UpstreamFailoverError{StatusCode: http.StatusBadGateway})
	require.False(t, state.skip(&service.AccountSelectionResult{Account: &otherRoute}, excluded), "unrelated failures must retain the normal account policy")
}
