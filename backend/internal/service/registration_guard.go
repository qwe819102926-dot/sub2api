package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/netip"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/redis/go-redis/v9"
)

const RegistrationTicketTTL = 10 * time.Minute

var (
	ErrRegistrationBrowser = infraerrors.Forbidden("REGISTRATION_BROWSER_REQUIRED", "Please complete registration in a web browser.")
	ErrRegistrationSession = infraerrors.Forbidden("REGISTRATION_SESSION_INVALID", "Registration session expired or changed. Please resend the verification code in this browser.")
	ErrRegistrationLimited = infraerrors.TooManyRequests("REGISTRATION_LIMITED", "Too many registration attempts. Please try again later or contact support.")
)

// RegistrationGuard binds email verification to an opaque, HttpOnly browser
// cookie. UA checks are only an additional signal, never proof of a human.
// Redis makes tickets, submission locks and quotas shared across instances.
type RegistrationGuard struct {
	redis *redis.Client
}

func NewRegistrationGuard(client *redis.Client) *RegistrationGuard {
	return &RegistrationGuard{redis: client}
}

func ValidateRegistrationBrowser(userAgent string) error {
	ua := strings.TrimSpace(userAgent)
	if len(ua) > 1024 || !strings.HasPrefix(ua, "Mozilla/5.0 ") ||
		(!strings.Contains(ua, "AppleWebKit/") && !strings.Contains(ua, "Gecko/")) {
		return ErrRegistrationBrowser
	}
	return nil
}

func registrationHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func registrationBinding(email, userAgent string) string {
	return registrationHash(strings.ToLower(strings.TrimSpace(email)) + "\x00" + userAgent)
}

// Count all quota dimensions atomically, without extending the window on each
// hit. Never put raw email addresses or browser cookies into Redis key names.
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

// ReserveAttempt protects mail delivery as well as registration. IPv6 address
// rotation within a /64 shares a quota. IPv4 subnet limits are deliberately
// higher to reduce impact on users sharing an ISP or corporate network.
func (g *RegistrationGuard) ReserveAttempt(ctx context.Context, email, clientIP string) error {
	if g == nil || g.redis == nil {
		return ErrServiceUnavailable
	}
	addr, err := netip.ParseAddr(clientIP)
	if err != nil {
		return ErrServiceUnavailable
	}
	addr = addr.Unmap()
	ipIdentity := addr.String()
	bits := 24
	if addr.Is6() {
		bits = 48
		ipIdentity = netip.PrefixFrom(addr, 64).Masked().String()
	}
	network := netip.PrefixFrom(addr, bits).Masked().String()
	keys := []string{
		"registration:quota:ip-short:" + registrationHash(ipIdentity),
		"registration:quota:ip-day:" + registrationHash(ipIdentity),
		"registration:quota:email-day:" + registrationHash(strings.ToLower(strings.TrimSpace(email))),
		"registration:quota:network-day:" + registrationHash(network),
	}
	allowed, err := registrationQuotaScript.Run(ctx, g.redis, keys,
		3, 600, 10, 86400, 5, 86400, 50, 86400).Int()
	if err != nil {
		return ErrServiceUnavailable
	}
	if allowed != 1 {
		return ErrRegistrationLimited
	}
	return nil
}

func registrationRandom() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", ErrServiceUnavailable
	}
	return hex.EncodeToString(buf[:]), nil
}

func registrationTicketKey(ticket string) string {
	return "registration:ticket:" + registrationHash(ticket)
}

// IssueTicket must only be called after successful captcha validation and mail
// enqueue. Tickets contain no account data and expire with the verification flow.
func (g *RegistrationGuard) IssueTicket(ctx context.Context, email, userAgent string) (string, error) {
	if g == nil || g.redis == nil {
		return "", ErrServiceUnavailable
	}
	ticket, err := registrationRandom()
	if err != nil {
		return "", err
	}
	if err := g.redis.Set(ctx, registrationTicketKey(ticket), registrationBinding(email, userAgent), RegistrationTicketTTL).Err(); err != nil {
		return "", ErrServiceUnavailable
	}
	return ticket, nil
}

var registrationAcquireScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
if not redis.call('SET', KEYS[2], ARGV[2], 'NX', 'EX', 120) then return 0 end
return 1
`)

// Acquire prevents concurrent reuse, but permits a retry after a wrong email
// code. The browser UA may not change; the IP may (e.g. mobile networks).
func (g *RegistrationGuard) Acquire(ctx context.Context, ticket, email, userAgent string) (string, error) {
	if g == nil || g.redis == nil {
		return "", ErrServiceUnavailable
	}
	if len(ticket) != 64 {
		return "", ErrRegistrationSession
	}
	if _, err := hex.DecodeString(ticket); err != nil {
		return "", ErrRegistrationSession
	}
	lease, err := registrationRandom()
	if err != nil {
		return "", err
	}
	key := registrationTicketKey(ticket)
	allowed, err := registrationAcquireScript.Run(ctx, g.redis, []string{key, key + ":lock"}, registrationBinding(email, userAgent), lease).Int()
	if err != nil {
		return "", ErrServiceUnavailable
	}
	if allowed != 1 {
		return "", ErrRegistrationSession
	}
	return lease, nil
}

var registrationFinishScript = redis.NewScript(`
if redis.call('GET', KEYS[2]) ~= ARGV[1] then return 0 end
redis.call('DEL', KEYS[2])
if ARGV[2] == '1' then redis.call('DEL', KEYS[1]) end
return 1
`)

// Finish only releases the matching lease, so a stale request cannot unlock a
// newer one. Successful registrations consume the ticket permanently.
func (g *RegistrationGuard) Finish(ctx context.Context, ticket, lease string, success bool) error {
	key := registrationTicketKey(ticket)
	consume := "0"
	if success {
		consume = "1"
	}
	if err := registrationFinishScript.Run(ctx, g.redis, []string{key, key + ":lock"}, lease, consume).Err(); err != nil {
		return ErrServiceUnavailable
	}
	return nil
}
