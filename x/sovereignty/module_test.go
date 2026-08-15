package sovereignty_test

import (
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func newTestModule(t *testing.T) (sovereignty.AppModule, *keeper.Keeper, sdk.Context, codec.JSONCodec) {
	t.Helper()
	storeService, ctx := colltest.MockStore()
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	k := keeper.NewKeeper(storeService, cdc)
	sdkCtx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger()).WithContext(ctx)
	mod := sovereignty.NewAppModule(cdc, k)
	return mod, k, sdkCtx, cdc
}

func TestAppModuleBasic_ValidateGenesis_AcceptsEveryRealState(t *testing.T) {
	_, _, _, cdc := newTestModule(t)
	basic := sovereignty.AppModuleBasic{}

	for _, state := range []string{types.StateAnchored, types.StateSuspicious, types.StateSovereign, types.StateRecovering} {
		t.Run(state, func(t *testing.T) {
			gs := types.DefaultGenesis()
			gs.FsmState = state
			bz := cdc.MustMarshalJSON(gs)
			require.NoError(t, basic.ValidateGenesis(cdc, nil, bz))
		})
	}
}

func TestAppModuleBasic_ValidateGenesis_RejectsUnknownState(t *testing.T) {
	_, _, _, cdc := newTestModule(t)
	basic := sovereignty.AppModuleBasic{}

	gs := types.DefaultGenesis()
	gs.FsmState = "NOT_A_REAL_STATE"
	bz := cdc.MustMarshalJSON(gs)
	require.ErrorContains(t, basic.ValidateGenesis(cdc, nil, bz), "invalid genesis fsm_state")
}

func TestAppModuleBasic_ValidateGenesis_RejectsMalformedJSON(t *testing.T) {
	_, _, _, cdc := newTestModule(t)
	basic := sovereignty.AppModuleBasic{}
	require.Error(t, basic.ValidateGenesis(cdc, nil, []byte("not json")))
}

// TestAppModule_InitGenesis_SetsEveryCounter guards against a real regression:
// UnhealthyStreak/FailedRecoveryAttempts/SuspiciousSafeBlocks are genesis
// fields (genesis.proto's fields 8-10, backing E5b/5c/backoff's TLA+
// counters) that InitGenesis silently dropped until this test was added --
// invisible at fresh genesis (Go zero-value already 0) but would have reset
// all 3 counters on every genesis export/import.
func TestAppModule_InitGenesis_SetsEveryCounter(t *testing.T) {
	mod, k, ctx, cdc := newTestModule(t)

	gs := types.DefaultGenesis()
	gs.FsmState = types.StateRecovering
	gs.SafeBlocksCounter = 3
	gs.SuspiciousDuration = 7
	gs.ReanchoringProofValid = true
	gs.UnhealthyStreak = 2
	gs.FailedRecoveryAttempts = 5
	gs.SuspiciousSafeBlocks = 4
	gs.InitialMetrics = &types.PeripheralMetrics{CleanPeers: 9}
	bz := cdc.MustMarshalJSON(gs)

	mod.InitGenesis(ctx, cdc, bz)

	fsmState, err := k.FSMState.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, types.StateRecovering, fsmState)

	safeBlocks, err := k.SafeBlocks.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(3), safeBlocks)

	suspiciousDuration, err := k.SuspiciousDuration.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(7), suspiciousDuration)

	proofValid, err := k.ReanchoringProofValid.Get(ctx)
	require.NoError(t, err)
	require.True(t, proofValid)

	unhealthyStreak, err := k.UnhealthyStreak.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(2), unhealthyStreak)

	failedRecoveryAttempts, err := k.FailedRecoveryAttempts.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(5), failedRecoveryAttempts)

	suspiciousSafeBlocks, err := k.SuspiciousSafeBlocks.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(4), suspiciousSafeBlocks)

	metrics, err := k.Metrics.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(9), metrics.CleanPeers)
}

// TestAppModule_InitGenesis_NilInitialMetricsLeavesMetricsUnset confirms
// InitGenesis's nil-guard around InitialMetrics: a genesis with no snapshot
// must not panic, and must leave Metrics unset (NewKeeper's own schema
// default), not a zero-value overwrite.
func TestAppModule_InitGenesis_NilInitialMetricsLeavesMetricsUnset(t *testing.T) {
	mod, k, ctx, cdc := newTestModule(t)

	gs := types.DefaultGenesis()
	gs.InitialMetrics = nil
	bz := cdc.MustMarshalJSON(gs)

	require.NotPanics(t, func() { mod.InitGenesis(ctx, cdc, bz) })

	_, err := k.Metrics.Get(ctx)
	require.Error(t, err, "Metrics must stay unset, not overwritten with a zero-value PeripheralMetrics")
}

// TestAppModule_ExportGenesis_RoundTripsInitGenesis confirms every field
// InitGenesis writes comes back out of ExportGenesis unchanged -- the
// property genesis export/import (e.g. a chain upgrade) depends on.
func TestAppModule_ExportGenesis_RoundTripsInitGenesis(t *testing.T) {
	mod, _, ctx, cdc := newTestModule(t)

	in := types.DefaultGenesis()
	in.FsmState = types.StateSovereign
	in.SafeBlocksCounter = 1
	in.SuspiciousDuration = 2
	in.ReanchoringProofValid = true
	in.UnhealthyStreak = 3
	in.FailedRecoveryAttempts = 4
	in.SuspiciousSafeBlocks = 5
	in.InitialMetrics = &types.PeripheralMetrics{CleanPeers: 6}
	mod.InitGenesis(ctx, cdc, cdc.MustMarshalJSON(in))

	outBz := mod.ExportGenesis(ctx, cdc)
	var out types.GenesisState
	require.NoError(t, cdc.UnmarshalJSON(outBz, &out))

	require.Equal(t, in.FsmState, out.FsmState)
	require.Equal(t, in.SafeBlocksCounter, out.SafeBlocksCounter)
	require.Equal(t, in.SuspiciousDuration, out.SuspiciousDuration)
	require.Equal(t, in.ReanchoringProofValid, out.ReanchoringProofValid)
	require.Equal(t, in.UnhealthyStreak, out.UnhealthyStreak)
	require.Equal(t, in.FailedRecoveryAttempts, out.FailedRecoveryAttempts)
	require.Equal(t, in.SuspiciousSafeBlocks, out.SuspiciousSafeBlocks)
	require.Equal(t, in.InitialMetrics.CleanPeers, out.InitialMetrics.CleanPeers)
}
