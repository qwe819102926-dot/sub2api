package repository

import "strings"

// usageCostExpression changes only the read projection used by user pages.
// Keep historical NULL wallet costs on the existing dashboard fallback, and
// exclude subscriptions because they do not deduct the principal wallet.
func usageCostExpression(principalOnly bool, prefix string) string {
	if !principalOnly {
		return prefix + "actual_cost"
	}
	return "CASE WHEN " + prefix + "billing_type = 0 THEN COALESCE(" + prefix + "wallet_cost, " + prefix + "actual_cost) ELSE 0 END"
}

func principalCostRequested(values []bool) bool {
	return len(values) > 0 && values[0]
}

func usageLogReadColumns(principalOnly bool) string {
	if !principalOnly {
		return usageLogSelectColumns
	}
	return strings.Replace(usageLogSelectColumns, ", actual_cost,", ", "+usageCostExpression(true, "")+" AS actual_cost,", 1)
}
