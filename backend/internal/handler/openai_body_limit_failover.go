package handler

import (
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Body limits belong to the upstream route, so rotating keys on that same route
// only repeats the oversized upload. This state lives for one client request;
// the normal group scheduler still controls eligibility on every selection.
type openAIRequestBodyFailoverState map[string]struct{}

func openAIRequestBodyUpstreamKey(account *service.Account) string {
	if account == nil || !account.IsOpenAIApiKey() {
		return ""
	}
	baseURL, err := url.Parse(strings.TrimSpace(account.GetOpenAIBaseURL()))
	if err != nil || baseURL.Host == "" {
		return ""
	}
	path := strings.TrimSuffix(strings.TrimRight(baseURL.Path, "/"), "/v1")
	return strings.ToLower(baseURL.Scheme) + "://" + strings.ToLower(baseURL.Host) + path
}

func (s openAIRequestBodyFailoverState) record(account *service.Account, err *service.UpstreamFailoverError) {
	if s == nil || err == nil || !err.IsOpenAIRequestBodyTooLarge() {
		return
	}
	if key := openAIRequestBodyUpstreamKey(account); key != "" {
		s[key] = struct{}{}
	}
}

func (s openAIRequestBodyFailoverState) skip(selection *service.AccountSelectionResult, excludedIDs map[int64]struct{}) bool {
	if selection == nil || selection.Account == nil || len(s) == 0 || excludedIDs == nil {
		return false
	}
	key := openAIRequestBodyUpstreamKey(selection.Account)
	if _, limited := s[key]; !limited || key == "" {
		return false
	}
	excludedIDs[selection.Account.ID] = struct{}{}
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
		selection.ReleaseFunc = nil
	}
	return true
}
