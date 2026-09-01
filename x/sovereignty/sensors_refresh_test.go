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

// TestRefreshReanchoringProofValid_WithinBoundedGapIsValid covers the real
// liveness bug this was added to fix: on a fast-producing testnet, a real,
// accepted proof's checkpoint is ALWAYS at least 1 block behind the tip by
// the time it lands (the committing block's own header is appended in
// PreBlocker, strictly after the proof's DeliverTx already ran) -- an exact
// match was found live to be structurally almost unreachable. A gap within
// Params.MaxUnprovenTailBlocks (default 8) must now count as valid.
func TestRefreshReanchoringProofValid_WithinBoundedGapIsValid(t *testing.T) {
	k, ctx := newRefreshTestKeeperCtx(t)
	setUpRecoveringWithProof(t, k, ctx, 18, 10) // gap = 8 = MaxUnprovenTailBlocks

	require.NoError(t, refreshReanchoringProofValid(ctx, k))
	valid, err := k.ReanchoringProofValid.Get(ctx)
	require.NoError(t, err)
	require.True(t, valid, "gap == MaxUnprovenTailBlocks must still be valid")
}

// TestRefreshReanchoringProofValid_BeyondBoundedGapIsInvalid confirms the
// bound is actually enforced, not accidentally unlimited -- a proof whose
// checkpoint has fallen further behind than Params.MaxUnprovenTailBlocks
// must NOT be treated as covering the current tip.
func TestRefreshReanchoringProofValid_BeyondBoundedGapIsInvalid(t *testing.T) {
	k, ctx := newRefreshTestKeeperCtx(t)
	setUpRecoveringWithProof(t, k, ctx, 19, 10) // gap = 9 > MaxUnprovenTailBlocks(8)

	require.NoError(t, refreshReanchoringProofValid(ctx, k))
	valid, err := k.ReanchoringProofValid.Get(ctx)
	require.NoError(t, err)
	require.False(t, valid, "gap beyond MaxUnprovenTailBlocks must be invalid -- this is a bound, not a bypass")
}

// TestRefreshReanchoringProofValid_ProofOnlyRecoveryDisablesHeuristic is
// E10's attribution fix: with ProofOnlyRecovery=true, a purely
// heuristic-valid state (h_btc_anchored >= h_btc_submitted, no real proof
// ever submitted) must NOT grant reanchoring_proof_valid -- otherwise a
// measured recovery can't be attributed to the ZK pipeline specifically.
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
// genesis-bootstrap deadlock (reproduced live on a fresh 4-node deploy):
// before Publisher.MaybePublish ever completes a submit+confirm cycle,
// VerifiedHeight() returns ok=false. daGapMetric used to fall back to
// hVerified=0 and compute gap=hCurrent-0, which reads as a small, healthy
// gap at low heights despite zero real DA activity -- IsDAHealthy=true then
// forced da.VerifyReceipt to require a real Attestation that couldn't exist
// yet, rejecting every proposal forever. A never-verified source must report
// the maximal gap instead, matching btcGapMetric's own fail-safe convention.
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
