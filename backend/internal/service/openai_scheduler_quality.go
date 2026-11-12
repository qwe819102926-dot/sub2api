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
	stat, ok := v.(*openAIAccountRuntimeStat)
	if !ok || stat == nil {
		return 0.5
	}
	// A fixed latency scale avoids one slow outlier distorting every score.
	return 0.5 + openAISchedulerFreshness(stat.lastTTFTAt.Load(), now)*(2000/(2000+math.Max(0, ms))-0.5)
}

func (s *openAIAccountRuntimeStats) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}
