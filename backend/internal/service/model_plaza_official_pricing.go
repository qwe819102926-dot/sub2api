package service

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
)

// ModelPlazaOfficialPriceOverride stores the public display rates in USD per million tokens.
// A nil field uses the default reference price for that field.
type ModelPlazaOfficialPriceOverride struct {
	Model             string   `json:"model"`
	InputPrice        *float64 `json:"input_price"`
	OutputPrice       *float64 `json:"output_price"`
	CacheWritePrice   *float64 `json:"cache_write_price"`
	CacheWrite1hPrice *float64 `json:"cache_write_1h_price"`
	CacheReadPrice    *float64 `json:"cache_read_price"`
}

func normalizeModelPlazaOfficialPriceOverrides(rows []ModelPlazaOfficialPriceOverride) ([]ModelPlazaOfficialPriceOverride, error) {
	normalized := make([]ModelPlazaOfficialPriceOverride, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		hasPrice := false
		for _, price := range []*float64{row.InputPrice, row.OutputPrice, row.CacheWritePrice, row.CacheWrite1hPrice, row.CacheReadPrice} {
			if price == nil {
				continue
			}
			if *price < 0 || math.IsNaN(*price) || math.IsInf(*price, 0) {
				return nil, fmt.Errorf("reference prices for %q must be finite and non-negative", row.Model)
			}
			hasPrice = true
		}
		if !hasPrice {
			continue
		}
		row.Model = strings.ToLower(strings.TrimSpace(row.Model))
		if row.Model == "" {
			return nil, fmt.Errorf("a model name is required for each reference price override")
		}
		if _, exists := seen[row.Model]; exists {
			return nil, fmt.Errorf("duplicate reference price override for %q", row.Model)
		}
		seen[row.Model] = struct{}{}
		normalized = append(normalized, row)
	}
	return normalized, nil
}

func parseModelPlazaOfficialPriceOverrides(raw string) []ModelPlazaOfficialPriceOverride {
	var rows []ModelPlazaOfficialPriceOverride
	if raw == "" || json.Unmarshal([]byte(raw), &rows) != nil || rows == nil {
		return []ModelPlazaOfficialPriceOverride{}
	}
	return rows
}

// DisplayPricing applies the configured fields to the reference price and each context tier.
// The model's shared default pricing stays untouched.
func (o ModelPlazaOfficialPriceOverride) DisplayPricing(defaults *PlazaOfficialPricing) *PlazaOfficialPricing {
	var result PlazaOfficialPricing
	if defaults != nil {
		result = *defaults
		result.Intervals = slices.Clone(defaults.Intervals)
	}
	input := plazaPricePerToken(o.InputPrice)
	output := plazaPricePerToken(o.OutputPrice)
	cacheWrite := plazaPricePerToken(o.CacheWritePrice)
	cacheWrite1h := plazaPricePerToken(o.CacheWrite1hPrice)
	cacheRead := plazaPricePerToken(o.CacheReadPrice)
	applyPlazaPriceOverride(&result.InputPrice, input)
	applyPlazaPriceOverride(&result.OutputPrice, output)
	applyPlazaPriceOverride(&result.CacheWritePrice, cacheWrite)
	applyPlazaPriceOverride(&result.CacheWrite1hPrice, cacheWrite1h)
	applyPlazaPriceOverride(&result.CacheReadPrice, cacheRead)
	// Keep flat cache rates together when only input and output have tiers.
	tieredCache := slices.ContainsFunc(result.Intervals, func(tier PricingInterval) bool {
		return tier.CacheWritePrice != nil || tier.CacheWrite1hPrice != nil || tier.CacheReadPrice != nil
	})
	for i := range result.Intervals {
		tier := &result.Intervals[i]
		applyPlazaPriceOverride(&tier.InputPrice, input)
		applyPlazaPriceOverride(&tier.OutputPrice, output)
		if tieredCache {
			applyPlazaPriceOverride(&tier.CacheWritePrice, cacheWrite)
			applyPlazaPriceOverride(&tier.CacheWrite1hPrice, cacheWrite1h)
			applyPlazaPriceOverride(&tier.CacheReadPrice, cacheRead)
		}
	}
	return &result
}

func plazaPricePerToken(price *float64) *float64 {
	if price == nil {
		return nil
	}
	value := *price / 1_000_000
	return &value
}

func applyPlazaPriceOverride(target **float64, override *float64) {
	if override != nil {
		*target = override
	}
}
