package sovereignty

import (
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// p2pDACleanMetrics returns metrics whose P2P and DA conditions are healthy
// (so critical stays false), with only BtcGap as the caller-controlled input --
// the minimal shape to test warning-grade transitions in isolation.
func p2pDACleanMetrics(p types.Params, btcGap uint64) *types.PeripheralMetrics {
	return &types.PeripheralMetrics{
		BtcGap:          btcGap,
		DaGap:           0,
		SubnetDiversity: p.MinSubnetDiversity,
		ActiveAnchors:   p.MinAnchorPeers,
		CleanPeers:      p.MinPeers,
		PeerChurnRate:   0,
		AvgPeerTenure:   p.MinAvgTenure,
		PeerLatency:     0,
	}
}

// TestBeginBlocker_WarningDemotesToSuspicious drives ANCHORED -> SUSPICIOUS:
// with default params, a warning-grade btc_gap demotes once UnhealthyStreak+1
// reaches DownHysteresisThreshold (E5's flapping fix).
func TestBeginBlocker_WarningDemotesToSuspicious(t *testing.T) {
	storeService, ctx := colltest.MockStore()
	k := keeper.NewKeeper(storeService, nil)
	sdkCtx := sdk.NewContext(nil, cmtproto.Header{Height: 42}, false, log.NewNopLogger()).WithContext(ctx)

	p := k.Params
	require.NoError(t, k.FSMState.Set(ctx, types.StateAnchored))
	require.NoError(t, k.UnhealthyStreak.Set(ctx, p.DownHysteresisThreshold-1))
	// btc_gap inside the SUSPICIOUS band [SuspiciousThreshold, SovereignThreshold).
	require.NoError(t, k.Metrics.Set(ctx, p2pDACleanMetrics(p, p.SuspiciousThreshold)))

	require.NoError(t, BeginBlocker(sdkCtx, k))

	state, err := k.FSMState.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, types.StateSuspicious, state)

	require.Equal(t, 1, len(sdkCtx.EventManager().Events()))
	ev := sdkCtx.EventManager().Events()[0]
	require.Equal(t, types.EventTypeFSMTransition, ev.Type)
	attrs := map[string]string{}
	for _, a := range ev.Attributes {
		attrs[string(a.Key)] = string(a.Value)
	}
	require.Equal(t, types.StateAnchored, attrs[types.AttributeKeyOldState])
	require.Equal(t, types.StateSuspicious, attrs[types.AttributeKeyNewState])
}

// TestBeginBlocker_AbsorbedWarningHoldsState: a warning with UnhealthyStreak
// below the down-hysteresis threshold is absorbed -- the chain stays ANCHORED
// and the streak advances toward the eventual demote.
func TestBeginBlocker_AbsorbedWarningHoldsState(t *testing.T) {
	storeService, ctx := colltest.MockStore()
	k := keeper.NewKeeper(storeService, nil)
	sdkCtx := sdk.NewContext(nil, cmtproto.Header{Height: 1}, false, log.NewNopLogger()).WithContext(ctx)

	p := k.Params
	require.NoError(t, k.FSMState.Set(ctx, types.StateAnchored))
	require.NoError(t, k.UnhealthyStreak.Set(ctx, 0))
	require.NoError(t, k.Metrics.Set(ctx, p2pDACleanMetrics(p, p.SuspiciousThreshold)))

	require.NoError(t, BeginBlocker(sdkCtx, k))

	state, err := k.FSMState.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, types.StateAnchored, state)
	require.Empty(t, sdkCtx.EventManager().Events(), "no state change -> no transition event")

	streak, err := k.UnhealthyStreak.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), streak)
}

// TestBeginBlocker_MissingStateDefaultsToAnchored covers BeginBlocker running
// before any genesis write: FSMState.Get fails, the code falls back to
// StateAnchored and healthy all-zero metrics produce no transition.
func TestBeginBlocker_MissingStateDefaultsToAnchored(t *testing.T) {
	storeService, ctx := colltest.MockStore()
	k := keeper.NewKeeper(storeService, nil)
	sdkCtx := sdk.NewContext(nil, cmtproto.Header{Height: 1}, false, log.NewNopLogger()).WithContext(ctx)

	require.NoError(t, k.Metrics.Set(ctx, p2pDACleanMetrics(k.Params, 0)))

	require.NoError(t, BeginBlocker(sdkCtx, k))

	// No transition happened (missing state defaulted to ANCHORED, healthy
	// metrics hold it), so nothing was persisted and no event was emitted.
	has, err := k.FSMState.Has(ctx)
	require.NoError(t, err)
	require.False(t, has)
	require.Empty(t, sdkCtx.EventManager().Events())
}
