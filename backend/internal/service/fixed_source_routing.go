package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
)

// FixedSourceRoute maps one API key source group to an account in a target group.
type FixedSourceRoute struct {
	SourceGroupID int64 `json:"source_group_id"`
	TargetGroupID int64 `json:"target_group_id"`
	AccountID     int64 `json:"account_id"`
}

// FixedSourceRoutingSettings contains global source indicators and group routes.
type FixedSourceRoutingSettings struct {
	Enabled bool               `json:"enabled"`
	Domains []string           `json:"domains"`
	IPs     []string           `json:"ips"`
	Routes  []FixedSourceRoute `json:"routes"`
}

func DefaultFixedSourceRoutingSettings() *FixedSourceRoutingSettings {
	return &FixedSourceRoutingSettings{Domains: []string{}, IPs: []string{}, Routes: []FixedSourceRoute{}}
}

func (s *SettingService) GetFixedSourceRoutingSettings(ctx context.Context) (*FixedSourceRoutingSettings, error) {
	if cached := s.fixedSourceRoutingCache.Load(); cached != nil {
		return cloneFixedSourceRoutingSettings(cached), nil
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyFixedSourceRoutingSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			settings := DefaultFixedSourceRoutingSettings()
			s.fixedSourceRoutingCache.Store(settings)
			return cloneFixedSourceRoutingSettings(settings), nil
		}
		return nil, fmt.Errorf("get fixed source routing settings: %w", err)
	}
	if strings.TrimSpace(value) == "" {
		settings := DefaultFixedSourceRoutingSettings()
		s.fixedSourceRoutingCache.Store(settings)
		return cloneFixedSourceRoutingSettings(settings), nil
	}
	settings := DefaultFixedSourceRoutingSettings()
	if err := json.Unmarshal([]byte(value), settings); err != nil {
		settings = DefaultFixedSourceRoutingSettings()
		s.fixedSourceRoutingCache.Store(settings)
		return cloneFixedSourceRoutingSettings(settings), nil
	}
	settings.Domains = normalizeFixedSourceDomains(settings.Domains)
	settings.IPs = normalizeFixedSourceIPs(settings.IPs)
	if settings.Routes == nil {
		settings.Routes = []FixedSourceRoute{}
	}
	s.fixedSourceRoutingCache.Store(settings)
	return cloneFixedSourceRoutingSettings(settings), nil
}

func (s *SettingService) SetFixedSourceRoutingSettings(ctx context.Context, settings *FixedSourceRoutingSettings) error {
	if settings == nil {
		return fmt.Errorf("fixed source routing settings cannot be nil")
	}
	if len(settings.Domains) > 100 || len(settings.IPs) > 100 || len(settings.Routes) > 200 {
		return fmt.Errorf("fixed source routing settings exceed the allowed size")
	}
	for _, value := range settings.Domains {
		if strings.TrimSpace(value) != "" && normalizeFixedSourceDomain(value) == "" {
			return fmt.Errorf("invalid fixed source domain %q", value)
		}
	}
	for _, value := range settings.IPs {
		if strings.TrimSpace(value) != "" && !isValidFixedSourceIP(value) {
			return fmt.Errorf("invalid fixed source IP or CIDR %q", value)
		}
	}
	settings.Domains = normalizeFixedSourceDomains(settings.Domains)
	settings.IPs = normalizeFixedSourceIPs(settings.IPs)
	if settings.Routes == nil {
		settings.Routes = []FixedSourceRoute{}
	}
	seen := make(map[int64]struct{}, len(settings.Routes))
	for _, route := range settings.Routes {
		if route.SourceGroupID <= 0 || route.TargetGroupID <= 0 || route.AccountID <= 0 {
			return fmt.Errorf("every fixed source route requires positive source_group_id, target_group_id, and account_id")
		}
		if _, ok := seen[route.SourceGroupID]; ok {
			return fmt.Errorf("source group %d has more than one fixed route", route.SourceGroupID)
		}
		seen[route.SourceGroupID] = struct{}{}
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal fixed source routing settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyFixedSourceRoutingSettings, string(data)); err != nil {
		return err
	}
	s.fixedSourceRoutingCache.Store(cloneFixedSourceRoutingSettings(settings))
	return nil
}

func cloneFixedSourceRoutingSettings(settings *FixedSourceRoutingSettings) *FixedSourceRoutingSettings {
	if settings == nil {
		return DefaultFixedSourceRoutingSettings()
	}
	return &FixedSourceRoutingSettings{
		Enabled: settings.Enabled,
		Domains: append([]string(nil), settings.Domains...),
		IPs:     append([]string(nil), settings.IPs...),
		Routes:  append([]FixedSourceRoute(nil), settings.Routes...),
	}
}

// GetFixedSourceRoutingTargetGroup resolves the configured target using the
// setting service's existing group dependency. Keeping this lookup here avoids
// widening every gateway route constructor solely for this optional feature.
func (s *SettingService) GetFixedSourceRoutingTargetGroup(ctx context.Context, groupID int64) (*Group, error) {
	if s == nil || s.defaultSubGroupReader == nil {
		return nil, ErrGroupNotFound
	}
	return s.defaultSubGroupReader.GetByID(ctx, groupID)
}

func normalizeFixedSourceDomains(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		domain := normalizeFixedSourceDomain(value)
		if domain == "" {
			continue
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		result = append(result, domain)
	}
	return result
}

func normalizeFixedSourceDomain(value string) string {
	value = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
	value = strings.TrimPrefix(value, "www.")
	if value == "" || len(value) > 253 || net.ParseIP(value) != nil {
		return ""
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || !isASCIILetterOrDigit(label[0]) || !isASCIILetterOrDigit(label[len(label)-1]) {
			return ""
		}
		for i := 1; i < len(label)-1; i++ {
			if !isASCIILetterOrDigit(label[i]) && label[i] != '-' {
				return ""
			}
		}
	}
	return value
}

func isASCIILetterOrDigit(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}

func normalizeFixedSourceIPs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !isValidFixedSourceIP(value) {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func isValidFixedSourceIP(value string) bool {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "/") {
		_, _, err := net.ParseCIDR(value)
		return err == nil
	}
	return net.ParseIP(value) != nil
}
