package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParamsValidate covers every branch of Params.Validate: each constraint
// returns its own error when violated, and the valid default passes.
func TestParamsValidate(t *testing.T) {
	t.Run("valid default passes", func(t *testing.T) {
		require.NoError(t, DefaultParams().Validate())
	})

	t.Run("KDeepFinality zero", func(t *testing.T) {
		p := DefaultParams()
		p.KDeepFinality = 0
		require.ErrorContains(t, p.Validate(), "KDeepFinality must be >= 1")
	})

	t.Run("SuspiciousThreshold not above KDeep+1", func(t *testing.T) {
		p := DefaultParams()
		p.SuspiciousThreshold = p.KDeepFinality + 1
		require.ErrorContains(t, p.Validate(), "SuspiciousThreshold")
	})

	t.Run("SovereignThreshold not above KDeep+1", func(t *testing.T) {
		p := DefaultParams()
		p.SovereignThreshold = p.KDeepFinality + 1
		require.ErrorContains(t, p.Validate(), "SovereignThreshold")
	})

	t.Run("SovereignThreshold not above SuspiciousThreshold", func(t *testing.T) {
		p := DefaultParams()
		p.SovereignThreshold = p.SuspiciousThreshold
		require.ErrorContains(t, p.Validate(), "SovereignThreshold")
	})

	t.Run("DAThreshold zero", func(t *testing.T) {
		p := DefaultParams()
		p.DAThreshold = 0
		require.ErrorContains(t, p.Validate(), "DAThreshold must be >= 1")
	})

	t.Run("MaxPeersPerSubnet zero", func(t *testing.T) {
		p := DefaultParams()
		p.MaxPeersPerSubnet = 0
		require.ErrorContains(t, p.Validate(), "MaxPeersPerSubnet must be >= 1")
	})

	t.Run("MaxDownHysteresisThreshold below DownHysteresisThreshold", func(t *testing.T) {
		p := DefaultParams()
		p.MaxDownHysteresisThreshold = p.DownHysteresisThreshold - 1
		require.ErrorContains(t, p.Validate(), "MaxDownHysteresisThreshold")
	})
}

// TestParamsRoundTrip verifies Params -> GenesisParams -> Params preserves
// every field, and that a nil GenesisParams falls back to DefaultParams().
func TestParamsRoundTrip(t *testing.T) {
	p := DefaultParams()
	gp := p.ToGenesisParams()
	require.Equal(t, p, gp.ToParams())

	require.Equal(t, DefaultParams(), (*GenesisParams)(nil).ToParams())
}
