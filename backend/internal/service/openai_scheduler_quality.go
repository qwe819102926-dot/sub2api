package service

import (
	"math"
	"time"
)

// Old observations lose half their influence every ten minutes. An account
// that stopped receiving traffic must not remain penalized indefinitely.
func openAISchedulerFreshness(observed int64, now time.Time) float64 {
	if observed <= 0 {
		return 0
	}
	age := now.Sub(time.Unix(0, observed))
	if age <= 0 {
		return 1
	}
	return math.Exp2(-float64(age) / float64(10*time.Minute))
}

func (s *openAIAccountRuntimeStats) latencyFactor(id int64, ms float64, now time.Time) float64 {
	if s == nil {
		return 0.5
	}
	v, ok := s.accounts.Load(id)
	if !ok {
		return 0.5
	}
	stat := v.(*openAIAccountRuntimeStat)
	// A fixed latency scale avoids one slow outlier distorting every score.
	return 0.5 + openAISchedulerFreshness(stat.lastTTFTAt.Load(), now)*(2000/(2000+math.Max(0, ms))-0.5)
}

func (s *openAIAccountRuntimeStats) reportCache(id int64, input, cached int) {
	if s == nil || input <= 0 || cached < 0 {
		return
	}
	stat := s.loadOrCreate(id)
	// Shrink small histories toward neutral instead of awarding a perfect
	// score to an account after a single cache hit.
	updateEWMAAtomic(&stat.cacheRatioBits, clamp01(float64(cached)/float64(input))-0.5, 0.2)
	stat.lastCacheAt.Store(time.Now().UnixNano())
}

func (s *openAIAccountRuntimeStats) cacheFactor(id int64, now time.Time) float64 {
	if s == nil {
		return 0.5
	}
	v, ok := s.accounts.Load(id)
	if !ok {
		return 0.5
	}
	stat := v.(*openAIAccountRuntimeStat)
	return clamp01(0.5 + openAISchedulerFreshness(stat.lastCacheAt.Load(), now)*math.Float64frombits(stat.cacheRatioBits.Load()))
}
