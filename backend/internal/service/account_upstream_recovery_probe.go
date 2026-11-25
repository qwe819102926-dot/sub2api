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

	"github.com/tidwall/gjson"
)

func (s *AccountTestService) runUpstreamRecoveryProbes(ctx context.Context) {
	repo, ok := s.accountRepo.(UpstreamRecoveryRepository)
	if !ok {
		return
	}
	ticker := time.NewTicker(upstreamRecoveryPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runDueUpstreamRecoveryProbes(ctx, repo)
		}
	}
}

func (s *AccountTestService) runDueUpstreamRecoveryProbes(ctx context.Context, repo UpstreamRecoveryRepository) {
	now := time.Now()
	claimCtx, cancel := context.WithTimeout(ctx, openAIAccountStateUpdateTimeout)
	claims, err := repo.ClaimUpstreamRecovery(claimCtx, 8, now, now.Add(upstreamRecoveryLease))
	cancel()
	if err != nil {
		slog.Error("upstream_recovery_claim_failed", "error", err)
		return
	}
	var workers sync.WaitGroup
	for _, claim := range claims {
		workers.Go(func() {
			probeCtx, cancel := context.WithTimeout(ctx, upstreamRecoveryProbeTimeout)
			err := s.probeUpstreamRecovery(probeCtx, claim)
			cancel()
			if ctx.Err() != nil {
				return
			}
			var retryAt time.Time
			if err != nil {
				retryAt = time.Now().Add(upstreamRecoveryCooldown)
				slog.Info("upstream_recovery_probe_failed", "account_id", claim.AccountID, "protocol", claim.State.Protocol, "reason", err.Error())
			}
			finishCtx, cancel := context.WithTimeout(ctx, openAIAccountStateUpdateTimeout)
			defer cancel()
			if err := repo.FinishUpstreamRecovery(finishCtx, claim, retryAt); err != nil {
				slog.Error("upstream_recovery_finish_failed", "account_id", claim.AccountID, "error", err)
			}
		})
	}
	workers.Wait()
}

func (s *AccountTestService) probeUpstreamRecovery(ctx context.Context, claim UpstreamRecoveryClaim) error {
	account, err := s.accountRepo.GetByID(ctx, claim.AccountID)
	if err != nil {
		return err
	}
	if !account.UpstreamRecoveryEnabled() || !account.IsActive() || !account.Schedulable {
		return fmt.Errorf("recovery is no longer active")
	}
	req, err := s.buildUpstreamRecoveryProbe(ctx, account, claim.State)
	if err != nil {
		return err
	}
	resp, err := s.httpUpstream.Do(req, resolveAccountProxyURL(account), account.ID, account.Concurrency)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return fmt.Errorf("probe transport failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("probe returned HTTP %d", resp.StatusCode)
	}
	return validateUpstreamRecoveryProbe(resp, claim.State.Protocol)
}

func (s *AccountTestService) buildUpstreamRecoveryProbe(ctx context.Context, account *Account, state UpstreamRecoveryState) (*http.Request, error) {
	base := account.GetOpenAIBaseURL()
	if state.Protocol == APIProtocolAnthropic {
		base = account.GetAnthropicProtocolBaseURL()
	} else if account.IsCNProvider() && account.IsAdaptiveAPIProtocol() && state.Protocol != APIProtocolChatCompletions {
		base = account.GetCNProtocolBaseURL(APIProtocolResponses)
	}
	base, err := s.validateUpstreamBaseURL(base)
	if err != nil {
		return nil, fmt.Errorf("invalid probe base URL")
	}

	text := "Reply with OK."
	messages := []map[string]string{{"role": "user", "content": text}}
	payload := map[string]any{"model": state.Model, "stream": state.Stream}
	var target string
	switch state.Protocol {
	case APIProtocolChatCompletions:
		target = buildOpenAIChatCompletionsURL(base)
		payload["messages"] = messages
	case APIProtocolAnthropic:
		target = strings.TrimRight(base, "/") + "/v1/messages"
		payload["messages"] = messages
		payload["max_tokens"] = 128
	case APIProtocolResponses:
		target = buildOpenAIResponsesURLForPlatform(account.Platform, base)
		payload["input"] = messages
		payload["max_output_tokens"] = 128
	case AccountTestModeCompact:
		target = strings.TrimRight(buildOpenAIResponsesURLForPlatform(account.Platform, base), "/") + "/compact"
		payload["input"] = messages
		delete(payload, "stream")
	default:
		return nil, fmt.Errorf("unsupported recovery protocol")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("invalid probe request URL")
	}
	key := strings.TrimSpace(account.GetOpenAIProtocolAPIKey())
	if key == "" {
		return nil, fmt.Errorf("missing probe API key")
	}
	if state.Protocol == APIProtocolAnthropic {
		setAnthropicAPIKeyAuthHeader(req.Header, account, key)
		setHeaderRaw(req.Header, "anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	req.Header.Set("Content-Type", "application/json")
	if state.Stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	account.ApplyHeaderOverrides(req.Header)
	return req, nil
}

func validateUpstreamRecoveryProbe(resp *http.Response, protocol string) error {
	reader := bufio.NewReaderSize(resp.Body, upstreamRecoveryBodyLimit)
	isSSE, err := upstreamRecoveryIsSSE(reader, resp.Header)
	if err != nil {
		return fmt.Errorf("empty probe response")
	}
	if isSSE {
		stream := &upstreamRecoveryStream{reader: reader, protocol: protocol}
		size := 0
		for size < upstreamRecoveryBodyLimit {
			line, err := stream.next()
			size += len(line)
			if stream.failed || err != nil {
				break
			}
			if stream.terminal {
				if stream.output {
					return nil
				}
				break
			}
		}
		return fmt.Errorf("probe did not return a complete model response")
	}

	body, err := io.ReadAll(io.LimitReader(reader, upstreamRecoveryBodyLimit+1))
	if err != nil || len(body) > upstreamRecoveryBodyLimit || !gjson.ValidBytes(body) {
		return fmt.Errorf("invalid probe response")
	}
	if !upstreamRecoveryValidJSON(body, protocol) {
		return fmt.Errorf("probe did not return a valid model response")
	}
	return nil
}
