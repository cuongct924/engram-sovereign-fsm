package types_test

import (
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

func TestParseBaselineMode(t *testing.T) {
	for in, want := range map[string]types.BaselineMode{
		"": types.BaselineEngram, "engram": types.BaselineEngram,
		"hard-gated": types.BaselineHardGated, "soft": types.BaselineSoft,
	} {
		got, err := types.ParseBaselineMode(in)
		require.NoError(t, err)
		require.Equal(t, want, got, in)
	}
	_, err := types.ParseBaselineMode("bogus")
	require.Error(t, err)
}

func TestBTCToleranceRound(t *testing.T) {
	require.Equal(t, uint64(50), types.BaselineEngram.BTCToleranceRound(50))
	require.Equal(t, uint64(50), types.BaselineSoft.BTCToleranceRound(50))
	require.Equal(t, uint64(3), types.BaselineHardGated.BTCToleranceRound(50))
	require.Equal(t, uint64(1), types.BaselineHardGated.BTCToleranceRound(1))
}
