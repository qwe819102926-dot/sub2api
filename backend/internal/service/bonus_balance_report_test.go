package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBonusBalanceReportFilter(t *testing.T) {
	f, err := normalizeBonusReportFilter(BonusBalanceReportFilter{Page: -1, PageSize: 1000, Search: "  42 "})
	require.NoError(t, err)
	require.Equal(t, "current", f.Scope)
	require.Equal(t, 1, f.Page)
	require.Equal(t, 20, f.PageSize)
	require.Equal(t, "42", f.Search)
	_, err = normalizeBonusReportFilter(BonusBalanceReportFilter{Source: "not-a-source"})
	require.Error(t, err)
	_, err = normalizeBonusReportFilter(BonusBalanceReportFilter{Kind: "invalid"})
	require.Error(t, err)
}

func TestBonusReportRejectsInvalidScopeAndDates(t *testing.T) {
	for _, f := range []BonusBalanceReportFilter{
		{Scope: "invalid"}, {StartDate: "2026-02-30"}, {EndDate: "2026-1-1"}, {StartDate: "2026-10-24", EndDate: "2026-10-23"},
	} {
		_, err := normalizeBonusReportFilter(f)
		require.Error(t, err)
	}
}
