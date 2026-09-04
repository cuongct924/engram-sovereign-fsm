package sovereignty

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/anchor"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper/sensors"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// stubBTCSource is a minimal sensors.BTCHeightSource for
// TestBtcGapMetric_AnchorNotDeepEnoughYetIsPendingNotFailed.
type stubBTCSource struct {
	height uint64
}

func (s stubBTCSource) CurrentHeight(ctx context.Context) (uint64, error) { return s.height, nil }
func (s stubBTCSource) Reachable(ctx context.Context) bool                { return true }

// newFakeBitcoindGetblockcount is a minimal stand-in bitcoind that only
// answers getblockcount -- enough to drive anchor.AnchorTracker's
// IsAnchorDeepEnough without a real node.
func newFakeBitcoindGetblockcount(t *testing.T, height int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID json.RawMessage `json:"id"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"id": req.ID, "result": height}))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

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

// TestBtcGapMetric_AnchorNotDeepEnoughYetIsPendingNotFailed is a regression
// test for a false-positive in committed sensor state: right after a fresh
// anchor submission (h_btc_anchored set, but not yet kDeepFinality-confirmed),
// btcGapMetric must not report spvFailed=true just because the depth check
// hasn't cleared yet -- that's pending, not a real tag-verification failure.
func TestBtcGapMetric_AnchorNotDeepEnoughYetIsPendingNotFailed(t *testing.T) {
	k, ctx := newRefreshTestKeeperCtx(t)
	require.NoError(t, k.HBtcAnchored.Set(ctx, 100))

	btcSrv := newFakeBitcoindGetblockcount(t, 101) // 101-100=1 < kDeepFinality=2
	tracker := anchor.NewAnchorTracker(anchor.NewRPCClient(btcSrv, "user", "pass"), 2)

	s := sensors.NewBTCSensor()
	s.SetSource(stubBTCSource{height: 101})

	gap, spvFailed, hCurrent, err := btcGapMetric(ctx, k, s, tracker)
	require.NoError(t, err)
	require.False(t, spvFailed, "an anchor short of kDeepFinality confirmations is pending, not a verification failure")
	require.Equal(t, uint64(1), gap)
	require.Equal(t, uint64(101), hCurrent)
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
