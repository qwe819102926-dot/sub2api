package service

import (
	"context"
	"strings"
)

// resolveAccountStatsCost 计算账号统计定价费用。
// 返回 nil 表示不覆盖，使用默认公式（total_cost × account_rate_multiplier）。
//
// 优先级（先命中为准）：
//  1. 自定义规则（始终尝试，不依赖 ApplyPricingToAccountStats 开关）
//  2. ApplyPricingToAccountStats 启用，且用户计费模型与上游模型相同（或 billedModel 为空）时，
//     直接使用本次请求的客户计费（倍率前的 totalCost）。
//     若请求已映射到不同上游模型，跳过复制，改按上游模型走优先级 3，
//     使管理员成本价与上游扣费口径对齐。
//  3. 模型定价文件（LiteLLM）中上游模型的默认价格
//  4. nil → 走默认公式（total_cost × account_rate_multiplier）
//
// 没有关联渠道时，映射到不同计费模型的请求仍走模型文件定价；同模型保留默认公式。
//
// upstreamModel 是最终发往上游的模型 ID。
// totalCost 是本次请求的客户计费（倍率前），用于优先级 2。
// serviceTier 是最终参与用户计费的 OpenAI 服务层级，用于优先级 3。
// billedModel 是实际用于用户计费的模型 ID。为空时保持历史行为（可复用 totalCost）。
// reasoningEffort 是最终转发等级；Fable 5.1 max 默认按 3 倍额度消耗。
func resolveAccountStatsCost(
	ctx context.Context,
	channelService *ChannelService,
	billingService *BillingService,
	accountID int64,
	groupID int64,
	upstreamModel string,
	tokens UsageTokens,
	requestCount int,
	totalCost float64,
	serviceTier string,
	billedModel string,
	reasoningEfforts ...string,
) *float64 {
	reasoningEffort := ""
	if len(reasoningEfforts) > 0 {
		reasoningEffort = reasoningEfforts[0]
	}
	if upstreamModel == "" {
		return nil
	}
	// Account-level mappings also work without a channel. In that case a
	// different upstream model still needs its own price, while unmapped
	// requests retain the existing total_cost fallback (including custom prices).
	if channelService == nil {
		return mappedAccountStatsFileCost(billingService, upstreamModel, billedModel, tokens, serviceTier, reasoningEffort)
	}
	channel, err := channelService.GetChannelForGroup(ctx, groupID)
	if err != nil {
		return nil
	}
	if channel == nil {
		return mappedAccountStatsFileCost(billingService, upstreamModel, billedModel, tokens, serviceTier, reasoningEffort)
	}

	platform := channelService.GetGroupPlatform(ctx, groupID)

	// 优先级 1：自定义规则（始终尝试）
	if cost := tryCustomRules(channel, accountID, groupID, platform, upstreamModel, tokens, requestCount, reasoningEffort); cost != nil {
		return cost
	}

	// 优先级 2：渠道开启"应用模型定价到账号统计"时，直接使用客户计费（倍率前）。
	// 计费模型与上游模型不同时不复制：用户价按请求模型算，管理员成本应按映射后上游模型算。
	if channel.ApplyPricingToAccountStats && accountStatsShouldReuseUserTotalCost(upstreamModel, billedModel) {
		cost := totalCost
		if cost <= 0 {
			return nil
		}
		return &cost
	}

	// 优先级 3：模型定价文件（LiteLLM）默认价格
	if billingService != nil {
		return tryModelFilePricing(billingService, upstreamModel, tokens, serviceTier, reasoningEffort)
	}

	return nil
}

func mappedAccountStatsFileCost(bs *BillingService, upstreamModel, billedModel string, tokens UsageTokens, serviceTier, reasoningEffort string) *float64 {
	if bs == nil || accountStatsShouldReuseUserTotalCost(upstreamModel, billedModel) {
		return nil
	}
	return tryModelFilePricing(bs, upstreamModel, tokens, serviceTier, reasoningEffort)
}

// accountStatsShouldReuseUserTotalCost reports whether account stats may copy the
// pre-multiplier user total. Empty billedModel keeps the historical copy path.
func accountStatsShouldReuseUserTotalCost(upstreamModel, billedModel string) bool {
	billedModel = strings.TrimSpace(billedModel)
	if billedModel == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(upstreamModel), billedModel)
}

// tryModelFilePricing 使用模型定价文件（LiteLLM/fallback）中的价格计算费用。
// 与用户计费共用同一条定价管线，避免这里维护第二份"单价 × token 数"实现后，
// 每加一个定价特性都要手工镜像一次。channelPricing 为 nil，保持优先级 3 的
// 语义：只取模型定价文件，不引入渠道自定义定价。
func tryModelFilePricing(billingService *BillingService, model string, tokens UsageTokens, serviceTier string, reasoningEfforts ...string) *float64 {
	reasoningEffort := ""
	if len(reasoningEfforts) > 0 {
		reasoningEffort = reasoningEfforts[0]
	}
	breakdown, err := billingService.CalculateCostWithServiceTier(
		model, tokens, 1, normalizeBillingServiceTier(serviceTier),
	)
	if err != nil || breakdown == nil || breakdown.TotalCost <= 0 {
		return nil
	}
	applyCostBreakdownMultiplier(breakdown, maxReasoningEffortBillingMultiplier(model, reasoningEffort, nil))
	return &breakdown.TotalCost
}

// tryCustomRules 遍历自定义规则，按数组顺序先命中为准。
func tryCustomRules(
	channel *Channel, accountID, groupID int64,
	platform, model string, tokens UsageTokens, requestCount int,
	reasoningEfforts ...string,
) *float64 {
	reasoningEffort := ""
	if len(reasoningEfforts) > 0 {
		reasoningEffort = reasoningEfforts[0]
	}
	modelLower := strings.ToLower(model)
	for _, rule := range channel.AccountStatsPricingRules {
		if !matchAccountStatsRule(&rule, accountID, groupID) {
			continue
		}
		pricing := findPricingForModel(rule.Pricing, platform, modelLower)
		if pricing == nil {
			continue // 规则匹配但模型不在规则定价中，继续下一条
		}
		cost := calculateStatsCost(pricing, tokens, requestCount)
		if cost != nil {
			*cost *= maxReasoningEffortBillingMultiplier(model, reasoningEffort, nil)
		}
		return cost
	}
	return nil
}

// matchAccountStatsRule 检查规则是否匹配指定的 accountID 和 groupID。
// 匹配条件：accountID ∈ rule.AccountIDs 或 groupID ∈ rule.GroupIDs。
// 如果规则的 AccountIDs 和 GroupIDs 都为空，视为不匹配。
func matchAccountStatsRule(rule *AccountStatsPricingRule, accountID, groupID int64) bool {
	if len(rule.AccountIDs) == 0 && len(rule.GroupIDs) == 0 {
		return false
	}
	for _, id := range rule.AccountIDs {
		if id == accountID {
			return true
		}
	}
	for _, id := range rule.GroupIDs {
		if id == groupID {
			return true
		}
	}
	return false
}

// findPricingForModel 在定价列表中查找匹配的模型定价。
// 先精确匹配，再通配符匹配（按配置顺序，先匹配先使用）。
func findPricingForModel(pricingList []ChannelModelPricing, platform, modelLower string) *ChannelModelPricing {
	// 精确匹配优先
	for i := range pricingList {
		p := &pricingList[i]
		if !isPlatformMatch(platform, p.Platform) {
			continue
		}
		for _, m := range p.Models {
			if strings.ToLower(m) == modelLower {
				return p
			}
		}
	}
	// 通配符匹配：按配置顺序，先匹配先使用
	for i := range pricingList {
		p := &pricingList[i]
		if !isPlatformMatch(platform, p.Platform) {
			continue
		}
		for _, m := range p.Models {
			ml := strings.ToLower(m)
			if !strings.HasSuffix(ml, "*") {
				continue
			}
			prefix := strings.TrimSuffix(ml, "*")
			if strings.HasPrefix(modelLower, prefix) {
				return p
			}
		}
	}
	return nil
}

// isPlatformMatch 判断平台是否匹配（空平台视为不限平台）。
func isPlatformMatch(queryPlatform, pricingPlatform string) bool {
	if queryPlatform == "" || pricingPlatform == "" {
		return true
	}
	return queryPlatform == pricingPlatform
}

// calculateStatsCost 使用给定的定价计算费用（不含任何倍率，原始费用）。
func calculateStatsCost(pricing *ChannelModelPricing, tokens UsageTokens, requestCount int) *float64 {
	if pricing == nil {
		return nil
	}
	switch pricing.BillingMode {
	case BillingModePerRequest, BillingModeImage:
		return calculatePerRequestStatsCost(pricing, requestCount)
	default:
		return calculateTokenStatsCost(pricing, tokens)
	}
}

// calculatePerRequestStatsCost 按次/图片计费。
func calculatePerRequestStatsCost(pricing *ChannelModelPricing, requestCount int) *float64 {
	if pricing.PerRequestPrice == nil || *pricing.PerRequestPrice <= 0 {
		return nil
	}
	cost := *pricing.PerRequestPrice * float64(requestCount)
	return &cost
}

// calculateTokenStatsCost Token 计费。
// If the pricing has intervals, find the matching interval by total token count
// and use its prices instead of the flat pricing fields.
func calculateTokenStatsCost(pricing *ChannelModelPricing, tokens UsageTokens) *float64 {
	p := pricing
	if len(pricing.Intervals) > 0 {
		totalTokens := tokens.InputTokens + tokens.OutputTokens + tokens.CacheCreationTokens + tokens.CacheReadTokens
		if iv := FindMatchingInterval(pricing.Intervals, totalTokens); iv != nil {
			p = &ChannelModelPricing{
				InputPrice:        iv.InputPrice,
				OutputPrice:       iv.OutputPrice,
				CacheWritePrice:   iv.CacheWritePrice,
				CacheWrite1hPrice: iv.CacheWrite1hPrice,
				CacheReadPrice:    iv.CacheReadPrice,
				PerRequestPrice:   iv.PerRequestPrice,
			}
		}
	}
	deref := func(ptr *float64) float64 {
		if ptr == nil {
			return 0
		}
		return *ptr
	}
	cacheCreationCost := float64(tokens.CacheCreationTokens) * deref(p.CacheWritePrice)
	if p.CacheWrite1hPrice != nil {
		cache5m, cache1h := normalizeCacheCreationBreakdown(tokens)
		if cache5m > 0 || cache1h > 0 {
			cacheCreationCost = float64(cache5m)*deref(p.CacheWritePrice) +
				float64(cache1h)*deref(p.CacheWrite1hPrice)
		}
	}
	cost := float64(tokens.InputTokens)*deref(p.InputPrice) +
		float64(tokens.OutputTokens)*deref(p.OutputPrice) +
		cacheCreationCost +
		float64(tokens.CacheReadTokens)*deref(p.CacheReadPrice) +
		float64(tokens.ImageOutputTokens)*deref(p.ImageOutputPrice)
	if cost <= 0 {
		return nil
	}
	return &cost
}

// resolveAccountStatsCostModel picks the model whose file/custom price should
// be used for admin account cost. Cost must follow the model actually sent
// upstream, not the client-billed model.
//
// /v1/responses passthrough often leaves result.UpstreamModel empty (or still
// the client model) while ChannelMappedModel / ModelMappingChain already record
// the rewrite shown in the admin UI as "gpt-5.5 -> gpt-5.6-terra".
func resolveAccountStatsCostModel(upstreamModel, channelMappedModel, originalModel, requestedModel, mappingChain string) string {
	original := firstNonEmpty(originalModel, requestedModel)
	if hop := lastDistinctMappingHop(mappingChain, original); hop != "" {
		return hop
	}
	mapped := strings.TrimSpace(channelMappedModel)
	if mapped != "" && original != "" && !strings.EqualFold(mapped, original) {
		return mapped
	}
	if upstream := strings.TrimSpace(upstreamModel); upstream != "" {
		return upstream
	}
	if mapped != "" {
		return mapped
	}
	if requested := strings.TrimSpace(requestedModel); requested != "" {
		return requested
	}
	return original
}

// lastDistinctMappingHop returns the last mapping-chain hop that is not the
// client-requested model. Walking backwards skips a trailing hop that wrongly
// repeats the original model (passthrough recording gpt-5.5->terra->gpt-5.5).
func lastDistinctMappingHop(chain, original string) string {
	chain = strings.TrimSpace(chain)
	if chain == "" || !strings.Contains(chain, "→") {
		return ""
	}
	original = strings.TrimSpace(original)
	parts := strings.Split(chain, "→")
	for i := len(parts) - 1; i >= 0; i-- {
		hop := strings.TrimSpace(parts[i])
		if hop == "" {
			continue
		}
		if original == "" || !strings.EqualFold(hop, original) {
			return hop
		}
	}
	return ""
}

// applyAccountStatsCost resolves the account stats cost for a usage log entry.
// It prefers the mapped upstream model (mapping chain / channel mapped model)
// over a passthrough UpstreamModel that is empty or still the client model.
// billedModel is the model actually used for user billing. When it differs from
// the upstream/mapped model, account stats skip copying user total_cost and
// price the upstream model instead.
func applyAccountStatsCost(
	ctx context.Context,
	usageLog *UsageLog,
	cs *ChannelService, bs *BillingService,
	accountID int64, groupID int64,
	upstreamModel, requestedModel, channelMappedModel string,
	tokens UsageTokens,
	totalCost float64,
	billedModel string,
) {
	original := requestedModel
	chain := ""
	mapped := channelMappedModel
	if usageLog != nil {
		if strings.TrimSpace(usageLog.RequestedModel) != "" {
			original = usageLog.RequestedModel
		}
		chain = optionalStringValue(usageLog.ModelMappingChain)
		if strings.TrimSpace(upstreamModel) == "" {
			upstreamModel = optionalStringValue(usageLog.UpstreamModel)
		}
		if strings.TrimSpace(requestedModel) == "" {
			requestedModel = usageLog.Model
		}
	}
	original = firstNonEmpty(original, requestedModel)
	model := resolveAccountStatsCostModel(upstreamModel, mapped, original, requestedModel, chain)
	if strings.TrimSpace(billedModel) == "" {
		billedModel = requestedModel
	}
	requestCount := 1
	if usageLog != nil && usageLog.ImageCount > 0 {
		requestCount = usageLog.ImageCount
	}
	serviceTier := ""
	reasoningEffort := ""
	if usageLog != nil && usageLog.ServiceTier != nil {
		serviceTier = *usageLog.ServiceTier
	}
	if usageLog != nil && usageLog.ReasoningEffort != nil {
		reasoningEffort = *usageLog.ReasoningEffort
	}
	usageLog.AccountStatsCost = resolveAccountStatsCost(
		ctx, cs, bs, accountID, groupID, model, tokens, requestCount, totalCost, serviceTier, billedModel, reasoningEffort,
	)
}
