package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBonusBalanceReportFilter(t *testing.T) {
	f, err := normalizeBonusReportFilter(BonusBalanceReportFilter{Page: -1, PageSize: 1000, Search: "  42 "})
	require.NoError(t, err)
	require.Equal(t, 1, f.Page)
	require.Equal(t, 20, f.PageSize)
	require.Equal(t, "42", f.Search)
	_, err = normalizeBonusReportFilter(BonusBalanceReportFilter{Source: "not-a-source"})
	require.Error(t, err)
	_, err = normalizeBonusReportFilter(BonusBalanceReportFilter{Kind: "invalid"})
	require.Error(t, err)
}
