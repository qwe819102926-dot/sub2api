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
	cache RegistrationGuardCache
}

// RegistrationGuardCache keeps atomic storage operations outside the service layer.
type RegistrationGuardCache interface {
	ReserveAttempts(context.Context, []RegistrationQuota) (bool, error)
	SetTicket(context.Context, string, string, time.Duration) error
	AcquireTicket(context.Context, string, string, string, time.Duration) (bool, error)
	FinishTicket(context.Context, string, string, bool) error
}

type RegistrationQuota struct {
	Key    string
	Limit  int
	Window time.Duration
}

func NewRegistrationGuard(cache RegistrationGuardCache) *RegistrationGuard {
	return &RegistrationGuard{cache: cache}
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

// ReserveAttempt protects mail delivery as well as registration. IPv6 address
// rotation within a /64 shares a quota. IPv4 subnet limits are deliberately
// higher to reduce impact on users sharing an ISP or corporate network.
func (g *RegistrationGuard) ReserveAttempt(ctx context.Context, email, clientIP string) error {
	if g == nil || g.cache == nil {
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
	allowed, err := g.cache.ReserveAttempts(ctx, []RegistrationQuota{
		{Key: keys[0], Limit: 3, Window: 10 * time.Minute},
		{Key: keys[1], Limit: 10, Window: 24 * time.Hour},
		{Key: keys[2], Limit: 5, Window: 24 * time.Hour},
		{Key: keys[3], Limit: 50, Window: 24 * time.Hour},
	})
	if err != nil {
		return ErrServiceUnavailable
	}
	if !allowed {
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
	if g == nil || g.cache == nil {
		return "", ErrServiceUnavailable
	}
	ticket, err := registrationRandom()
	if err != nil {
		return "", err
	}
	if err := g.cache.SetTicket(ctx, registrationTicketKey(ticket), registrationBinding(email, userAgent), RegistrationTicketTTL); err != nil {
		return "", ErrServiceUnavailable
	}
	return ticket, nil
}

// Acquire prevents concurrent reuse, but permits a retry after a wrong email
// code. The browser UA may not change; the IP may (e.g. mobile networks).
func (g *RegistrationGuard) Acquire(ctx context.Context, ticket, email, userAgent string) (string, error) {
	if g == nil || g.cache == nil {
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
	allowed, err := g.cache.AcquireTicket(ctx, key, registrationBinding(email, userAgent), lease, 2*time.Minute)
	if err != nil {
		return "", ErrServiceUnavailable
	}
	if !allowed {
		return "", ErrRegistrationSession
	}
	return lease, nil
}

// Finish only releases the matching lease, so a stale request cannot unlock a
// newer one. Successful registrations consume the ticket permanently.
func (g *RegistrationGuard) Finish(ctx context.Context, ticket, lease string, success bool) error {
	if g == nil || g.cache == nil {
		return ErrServiceUnavailable
	}
	if err := g.cache.FinishTicket(ctx, registrationTicketKey(ticket), lease, success); err != nil {
		return ErrServiceUnavailable
	}
	return nil
}
