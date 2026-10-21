package repository

import "strings"

// usageCostExpression changes only the read projection used by user pages.
// Balance charges show principal deductions only; subscriptions keep their
// original billed cost. Historical NULL wallet costs retain the existing fallback.
func usageCostExpression(principalOnly bool, prefix string) string {
	if !principalOnly {
		return prefix + "actual_cost"
	}
	return "CASE WHEN " + prefix + "billing_type = 0 THEN COALESCE(" + prefix + "wallet_cost, " + prefix + "actual_cost) ELSE " + prefix + "actual_cost END"
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
