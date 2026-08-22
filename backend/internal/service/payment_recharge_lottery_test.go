package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateRechargeLotteryConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  RechargeLotteryConfig
		wantErr bool
	}{
		{
			name: "valid enabled campaign",
			config: RechargeLotteryConfig{
				Enabled:   true,
				Threshold: 10,
				Prizes: []RechargeLotteryPrize{
					{Amount: 3, Probability: 20},
					{Amount: 5, Probability: 30},
				},
			},
		},
		{
			name:    "enabled campaign requires threshold",
			config:  RechargeLotteryConfig{Enabled: true, Prizes: []RechargeLotteryPrize{{Amount: 1, Probability: 1}}},
			wantErr: true,
		},
		{
			name:    "enabled campaign requires prizes",
			config:  RechargeLotteryConfig{Enabled: true, Threshold: 10},
			wantErr: true,
		},
		{
			name: "probability may not exceed one hundred percent",
			config: RechargeLotteryConfig{Enabled: true, Threshold: 10, Prizes: []RechargeLotteryPrize{
				{Amount: 1, Probability: 60},
				{Amount: 2, Probability: 50},
			}},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRechargeLotteryConfig(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestSelectRechargeLotteryPrizeWithCertainPrize(t *testing.T) {
	prizes := []RechargeLotteryPrize{{Amount: 7.5, Probability: 100}}
	prize, err := selectRechargeLotteryPrize(prizes)
	require.NoError(t, err)
	require.NotNil(t, prize)
	require.Equal(t, 7.5, prize.Amount)
}
