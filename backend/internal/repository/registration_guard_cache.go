package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

type registrationGuardCache struct {
	rdb *redis.Client
}

func NewRegistrationGuardCache(rdb *redis.Client) service.RegistrationGuardCache {
	return &registrationGuardCache{rdb: rdb}
}

var errRegistrationCacheUnavailable = errors.New("registration cache unavailable")

// Check every quota before incrementing any of them. Windows start on first use
// and must not be extended by subsequent requests.
var registrationQuotaScript = redis.NewScript(`
for i, key in ipairs(KEYS) do
  if tonumber(redis.call('GET', key) or '0') >= tonumber(ARGV[(i-1)*2+1]) then
    return 0
  end
end
for i, key in ipairs(KEYS) do
  if redis.call('INCR', key) == 1 then
    redis.call('EXPIRE', key, ARGV[(i-1)*2+2])
  end
end
return 1
`)

func (c *registrationGuardCache) ReserveAttempts(ctx context.Context, quotas []service.RegistrationQuota) (bool, error) {
	if c.rdb == nil {
		return false, errRegistrationCacheUnavailable
	}
	keys := make([]string, 0, len(quotas))
	args := make([]any, 0, len(quotas)*2)
	for _, quota := range quotas {
		keys = append(keys, quota.Key)
		args = append(args, quota.Limit, int64(quota.Window/time.Second))
	}
	allowed, err := registrationQuotaScript.Run(ctx, c.rdb, keys, args...).Int()
	return allowed == 1, err
}

func (c *registrationGuardCache) SetTicket(ctx context.Context, key, binding string, ttl time.Duration) error {
	if c.rdb == nil {
		return errRegistrationCacheUnavailable
	}
	return c.rdb.Set(ctx, key, binding, ttl).Err()
}

var registrationAcquireScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
if not redis.call('SET', KEYS[2], ARGV[2], 'NX', 'EX', ARGV[3]) then return 0 end
return 1
`)

func (c *registrationGuardCache) AcquireTicket(ctx context.Context, key, binding, lease string, ttl time.Duration) (bool, error) {
	if c.rdb == nil {
		return false, errRegistrationCacheUnavailable
	}
	allowed, err := registrationAcquireScript.Run(ctx, c.rdb, []string{key, key + ":lock"}, binding, lease, int64(ttl/time.Second)).Int()
	return allowed == 1, err
}

var registrationFinishScript = redis.NewScript(`
if redis.call('GET', KEYS[2]) ~= ARGV[1] then return 0 end
redis.call('DEL', KEYS[2])
if ARGV[2] == '1' then redis.call('DEL', KEYS[1]) end
return 1
`)

func (c *registrationGuardCache) FinishTicket(ctx context.Context, key, lease string, success bool) error {
	if c.rdb == nil {
		return errRegistrationCacheUnavailable
	}
	consume := "0"
	if success {
		consume = "1"
	}
	return registrationFinishScript.Run(ctx, c.rdb, []string{key, key + ":lock"}, lease, consume).Err()
}
