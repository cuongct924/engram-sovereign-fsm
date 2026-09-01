package sovereignty

import (
	"context"
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper/sensors"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// stubDASource is a minimal sensors.DAAvailabilitySource for
// TestDaGapMetric_NeverVerifiedReportsMaxGap.
type stubDASource struct {
	height uint64
	ok     bool
}

func (s stubDASource) VerifiedHeight() (uint64, bool)        { return s.height, s.ok }
func (s stubDASource) Failed() bool                          { return false }
func (s stubDASource) ProbeHealthy(ctx context.Context) bool { return true }

func newRefreshTestKeeperCtx(t *testing.T) (*keeper.Keeper, sdk.Context) {
	t.Helper()
	storeService, ctx := colltest.MockStore()
	k := keeper.NewKeeper(storeService, nil)
	sdkCtx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger()).WithContext(ctx)
	return k, sdkCtx
}

// setUpRecoveringWithProof puts k into RECOVERING with a tracked interval
// [1..tipHeight] and a real-proof latch pointing at proofHeight (<=
// tipHeight), mirroring what CommitFSMTransition + a real accepted
// SubmitRecoveryProof leave behind.
func setUpRecoveringWithProof(t *testing.T, k *keeper.Keeper, ctx sdk.Context, tipHeight, proofHeight uint64) {
	t.Helper()
	require.NoError(t, k.FSMState.Set(ctx, types.StateRecovering))
	for h := proofHeight + 1; h <= tipHeight; h++ {
		require.NoError(t, k.HeaderHistory.Set(ctx, h, types.RecoveryHeader{FsmState: types.StateRecovering, StateRoot: []byte{byte(h)}}))
	}
	require.NoError(t, k.RealProofSubmittedHeight.Set(ctx, proofHeight))
}

// TestRefreshReanchoringProofValid_ExactMatchStillValid is the gap=0
// boundary: a proof whose checkpoint IS the tip must still count valid
// (the bounded-gap relaxation must not regress the previously-working case).
func TestRefreshReanchoringProofValid_ExactMatchStillValid(t *testing.T) {
	k, ctx := newRefreshTestKeeperCtx(t)
	require.NoError(t, k.HeaderHistory.Set(ctx, 10, types.RecoveryHeader{FsmState: types.StateRecovering, StateRoot: []byte("tip")}))
	require.NoError(t, k.FSMState.Set(ctx, types.StateRecovering))
	require.NoError(t, k.RealProofSubmittedHeight.Set(ctx, 10))

	require.NoError(t, refreshReanchoringProofValid(ctx, k))
	valid, err := k.ReanchoringProofValid.Get(ctx)
	require.NoError(t, err)
	require.True(t, valid, "gap=0 (exact match) must still be valid")
}

// TestRefreshReanchoringProofValid_WithinBoundedGapIsValid covers a real
// liveness bug: a real accepted proof's checkpoint is always at least 1
// block behind the tip by the time it lands, so an exact match is almost
// unreachable live. A gap within Params.MaxUnprovenTailBlocks must be valid.
func TestRefreshReanchoringProofValid_WithinBoundedGapIsValid(t *testing.T) {
	k, ctx := newRefreshTestKeeperCtx(t)
	setUpRecoveringWithProof(t, k, ctx, 18, 10) // gap = 8 = MaxUnprovenTailBlocks

	require.NoError(t, refreshReanchoringProofValid(ctx, k))
	valid, err := k.ReanchoringProofValid.Get(ctx)
	require.NoError(t, err)
	require.True(t, valid, "gap == MaxUnprovenTailBlocks must still be valid")
}

// TestRefreshReanchoringProofValid_BeyondBoundedGapIsInvalid confirms the
// bound is actually enforced: a checkpoint further behind than
// MaxUnprovenTailBlocks must NOT be treated as covering the current tip.
func TestRefreshReanchoringProofValid_BeyondBoundedGapIsInvalid(t *testing.T) {
	k, ctx := newRefreshTestKeeperCtx(t)
	setUpRecoveringWithProof(t, k, ctx, 19, 10) // gap = 9 > MaxUnprovenTailBlocks(8)

	require.NoError(t, refreshReanchoringProofValid(ctx, k))
	valid, err := k.ReanchoringProofValid.Get(ctx)
	require.NoError(t, err)
	require.False(t, valid, "gap beyond MaxUnprovenTailBlocks must be invalid -- this is a bound, not a bypass")
}

// TestRefreshReanchoringProofValid_ProofOnlyRecoveryDisablesHeuristic: with
// ProofOnlyRecovery=true, a purely heuristic-valid state (no real proof)
// must NOT grant reanchoring_proof_valid.
func TestRefreshReanchoringProofValid_ProofOnlyRecoveryDisablesHeuristic(t *testing.T) {
	k, ctx := newRefreshTestKeeperCtx(t)
	require.NoError(t, k.FSMState.Set(ctx, types.StateRecovering))
	require.NoError(t, k.HBtcSubmitted.Set(ctx, 5))
	require.NoError(t, k.HBtcAnchored.Set(ctx, 5)) // heuristicValid: hAnchored >= hSubmitted > 0

	require.NoError(t, refreshReanchoringProofValid(ctx, k))
	valid, err := k.ReanchoringProofValid.Get(ctx)
	require.NoError(t, err)
	require.True(t, valid, "heuristic alone must still grant validity when ProofOnlyRecovery is unset (default)")

	k.Params.ProofOnlyRecovery = true
	require.NoError(t, refreshReanchoringProofValid(ctx, k))
	valid, err = k.ReanchoringProofValid.Get(ctx)
	require.NoError(t, err)
	require.False(t, valid, "ProofOnlyRecovery=true must reject a purely heuristic-valid state with no real proof")
}

// TestRefreshReanchoringProofValid_ProofOnlyRecoveryStillAcceptsRealProof
// confirms ProofOnlyRecovery doesn't break the real-proof path it exists to
// isolate -- only the heuristic fallback is disabled.
func TestRefreshReanchoringProofValid_ProofOnlyRecoveryStillAcceptsRealProof(t *testing.T) {
	k, ctx := newRefreshTestKeeperCtx(t)
	k.Params.ProofOnlyRecovery = true
	setUpRecoveringWithProof(t, k, ctx, 18, 10) // gap = 8 = MaxUnprovenTailBlocks, no heuristic fields set

	require.NoError(t, refreshReanchoringProofValid(ctx, k))
	valid, err := k.ReanchoringProofValid.Get(ctx)
	require.NoError(t, err)
	require.True(t, valid, "a real, bounded-gap proof must still grant validity under ProofOnlyRecovery")
}

// TestDaGapMetric_NeverVerifiedReportsMaxGap is a regression test for a real
// genesis-bootstrap deadlock: before any submit+confirm cycle completes,
// falling back to gap=hCurrent-0 reads as healthy and rejected every
// proposal forever. A never-verified source must report the maximal gap.
func TestDaGapMetric_NeverVerifiedReportsMaxGap(t *testing.T) {
	k, ctx := newRefreshTestKeeperCtx(t)
	ctx = ctx.WithBlockHeight(2) // low height -- hCurrent-0 would read as a tiny, healthy gap

	s := sensors.NewDASensor()
	s.SetSource(stubDASource{ok: false})

	gap, dasFailed, attestationFailed, err := daGapMetric(ctx, k, s)
	require.NoError(t, err)
	require.False(t, dasFailed)
	require.False(t, attestationFailed)
	require.GreaterOrEqual(t, gap, k.Params.DAThreshold,
		"a source that has never verified anything must report a gap >= DAThreshold (unhealthy), not hCurrent-0")
}
