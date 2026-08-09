package sovereignty

import (
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/iden3/go-merkletree-sql/v2/db/memory"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func newRefreshTestKeeperCtx(t *testing.T) (*keeper.Keeper, sdk.Context) {
	t.Helper()
	storeService, ctx := colltest.MockStore()
	k := keeper.NewKeeper(storeService, nil, memory.NewMemoryStorage())
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
// Params.MaxUnprovenTailBlocks (default 4) must now count as valid.
func TestRefreshReanchoringProofValid_WithinBoundedGapIsValid(t *testing.T) {
	k, ctx := newRefreshTestKeeperCtx(t)
	setUpRecoveringWithProof(t, k, ctx, 26, 10) // gap = 16 = MaxUnprovenTailBlocks

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
	setUpRecoveringWithProof(t, k, ctx, 27, 10) // gap = 17 > MaxUnprovenTailBlocks(16)

	require.NoError(t, refreshReanchoringProofValid(ctx, k))
	valid, err := k.ReanchoringProofValid.Get(ctx)
	require.NoError(t, err)
	require.False(t, valid, "gap beyond MaxUnprovenTailBlocks must be invalid -- this is a bound, not a bypass")
}
