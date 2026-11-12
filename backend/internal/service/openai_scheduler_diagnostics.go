package service

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

const schedulerDiagnosticRows = 200

// Diagnostics contain only routing metadata, never session identifiers, prompts,
// credentials or raw upstream errors. They describe selection, not completion.
type SchedulerDiagnosticAccount struct {
	ID               int64              `json:"id"`
	Name             string             `json:"name"`
	Reason           string             `json:"reason,omitempty"`
	Rate             *float64           `json:"rate"`
	Score            *float64           `json:"score"`
	Parts            map[string]float64 `json:"parts,omitempty"`
	TTFT             *float64           `json:"ttft_ms"`
	ErrorRate        float64            `json:"error_rate"`
	LatencyFreshness float64            `json:"latency_freshness"`
	Order            int                `json:"order"`
}

type SchedulerDiagnostic struct {
	indexes        map[int64]int
	ID             uint64                       `json:"id"`
	At             time.Time                    `json:"at"`
	GroupID        int64                        `json:"group_id"`
	Model          string                       `json:"model"`
	Layer          string                       `json:"layer"`
	SelectedID     int64                        `json:"selected_id"`
	TopK           int                          `json:"top_k"`
	CostEnabled    bool                         `json:"cost_enabled"`
	StickyWeighted bool                         `json:"sticky_weighted"`
	RequireCompact bool                         `json:"require_compact"`
	Truncated      bool                         `json:"truncated"`
	Accounts       []SchedulerDiagnosticAccount `json:"accounts"`
}

type schedulerDiagnosticKey struct{}
type schedulerDiagnosticStore struct {
	mu     sync.Mutex
	traces []SchedulerDiagnostic
}

var recentSchedulerDiagnostics schedulerDiagnosticStore
var schedulerDiagnosticSequence atomic.Uint64

func (s *schedulerDiagnosticStore) add(d SchedulerDiagnostic) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.traces) >= 100 {
		copy(s.traces, s.traces[1:])
		s.traces = s.traces[:99]
	}
	s.traces = append(s.traces, d)
}

func (s *schedulerDiagnosticStore) list(group int64, now time.Time) []SchedulerDiagnostic {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SchedulerDiagnostic, 0)
	for i := len(s.traces) - 1; i >= 0; i-- {
		d := s.traces[i]
		if now.Sub(d.At) > 15*time.Minute || (group > 0 && d.GroupID != group) {
			continue
		}
		out = append(out, d)
		if len(out) >= 30 {
			break
		}
	}
	return out
}

func RecentSchedulerDiagnostics(group int64) []SchedulerDiagnostic {
	return recentSchedulerDiagnostics.list(group, time.Now())
}

func schedulerDiagnosticFromContext(ctx context.Context) *SchedulerDiagnostic {
	d, _ := ctx.Value(schedulerDiagnosticKey{}).(*SchedulerDiagnostic)
	return d
}

func (d *SchedulerDiagnostic) canRecord(id int64) bool {
	if d.indexes == nil {
		d.indexes = make(map[int64]int)
		for i, row := range d.Accounts {
			d.indexes[row.ID] = i
		}
	}
	if _, ok := d.indexes[id]; ok {
		return true
	}
	if len(d.Accounts) >= schedulerDiagnosticRows {
		d.Truncated = true
		return false
	}
	return true
}

func (d *SchedulerDiagnostic) account(row SchedulerDiagnosticAccount) {
	if !d.canRecord(row.ID) {
		return
	}
	if i, ok := d.indexes[row.ID]; ok {
		d.Accounts[i] = row
		return
	}
	d.indexes[row.ID] = len(d.Accounts)
	d.Accounts = append(d.Accounts, row)
}

func recordSchedulerDiagnosticExclusion(ctx context.Context, a *Account, reason string) {
	if d := schedulerDiagnosticFromContext(ctx); d != nil && d.canRecord(a.ID) {
		if i, ok := d.indexes[a.ID]; ok {
			d.Accounts[i].Reason = reason
			return
		}
		d.account(SchedulerDiagnosticAccount{ID: a.ID, Name: a.Name, Reason: reason})
	}
}

func (s *defaultOpenAIAccountScheduler) recordDiagnosticScore(ctx context.Context, req OpenAIAccountScheduleRequest, item openAIAccountCandidateScore, parts map[string]float64, now time.Time) {
	d := schedulerDiagnosticFromContext(ctx)
	if d == nil {
		return
	}
	row := SchedulerDiagnosticAccount{ID: item.account.ID, Name: item.account.Name, Score: &item.score, Parts: parts, ErrorRate: item.errorRate}
	if req.UseUpstreamTokenCost {
		if rate, ok := openAISchedulingRate(item.account, now, s.service.openAIOAuthSchedulingRateMultiplier(ctx)); ok {
			row.Rate = &rate
		}
	}
	if item.hasTTFT {
		row.TTFT = &item.ttft
		if s.stats != nil {
			if v, ok := s.stats.accounts.Load(item.account.ID); ok {
				row.LatencyFreshness = openAISchedulerFreshness(v.(*openAIAccountRuntimeStat).lastTTFTAt.Load(), now)
			}
		}
	}
	d.account(row)
}
