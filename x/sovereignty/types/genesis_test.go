package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultGenesis(t *testing.T) {
	gs := DefaultGenesis()
	require.Equal(t, StateAnchored, gs.FsmState)
	require.Zero(t, gs.SafeBlocksCounter)
	require.Zero(t, gs.SuspiciousDuration)
	require.False(t, gs.ReanchoringProofValid)
	require.NotNil(t, gs.InitialMetrics)
	require.Equal(t, DefaultParams(), gs.Params.ToParams())
}

func TestDefaultGenesisWithParams(t *testing.T) {
	p := DefaultParams()
	p.KDeepFinality = 6
	gs := DefaultGenesisWithParams(p)
	require.Equal(t, StateAnchored, gs.FsmState)
	require.Equal(t, p, gs.Params.ToParams())
}
