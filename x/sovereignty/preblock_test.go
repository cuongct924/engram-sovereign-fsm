package sovereignty_test

import (
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/anchor"
	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// newTestKeeperCtxAt mirrors newTestKeeperCtx but with a caller-controlled
// height/AppHash, since Step 6's HeaderHistory/LastAnchoredRoot writes key off both.
func newTestKeeperCtxAt(t *testing.T, height int64, appHash []byte) (*keeper.Keeper, sdk.Context) {
	t.Helper()
	storeService, ctx := colltest.MockStore()
	k := keeper.NewKeeper(storeService, nil)
	sdkCtx := sdk.NewContext(nil, cmtproto.Header{Height: height, AppHash: appHash}, false, log.NewNopLogger()).WithContext(ctx)
	return k, sdkCtx
}

func lockedExt(state string) sovereignty.ExtendedProposal {
	return sovereignty.ExtendedProposal{
		FSMState:   state,
		DAReceipt:  da.Receipt{PublishedBlockHeight: 1, Attestation: true},
		BTCReceipt: anchor.Receipt{CheckpointBlockHeight: 0, CheckpointBlockHash: anchor.ExpectedBlockHash(0)},
	}
}

// TestCommitFSMTransition_HeaderHistory_FirstEntryWritesLastAnchoredRoot
// covers ANCHORED -> SOVEREIGN: latches LastAnchoredRoot to the incoming
// block's AppHash and writes this block's own HeaderHistory entry.
func TestCommitFSMTransition_HeaderHistory_FirstEntryWritesLastAnchoredRoot(t *testing.T) {
	appHash := []byte{0xAA, 0xBB, 0xCC}
	k, ctx := newTestKeeperCtxAt(t, 100, appHash)
	// currState defaults to ANCHORED (FSMState unset) -- CommitFSMTransition's
	// own fallback, matching a fresh chain.

	require.NoError(t, sovereignty.CommitFSMTransition(ctx, k, lockedExt(types.StateSovereign)))

	root, err := k.LastAnchoredRoot.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, types.ReduceToField(appHash), root)

	header, err := k.HeaderHistory.Get(ctx, 100)
	require.NoError(t, err)
	require.Equal(t, types.RecoveryHeader{
		FsmState:         types.StateSovereign,
		WithdrawalLocked: true,
		StateRoot:        types.ReduceToField(appHash),
	}, header)
}

// TestCommitFSMTransition_HeaderHistory_ContinuingIntervalPreservesLastAnchoredRoot
// covers staying locked (SOVEREIGN -> RECOVERING): a new HeaderHistory entry
// is written, but LastAnchoredRoot (rt_last) must not move.
func TestCommitFSMTransition_HeaderHistory_ContinuingIntervalPreservesLastAnchoredRoot(t *testing.T) {
	k, ctx := newTestKeeperCtxAt(t, 101, []byte{0xDE, 0xAD})
	require.NoError(t, k.FSMState.Set(ctx, types.StateSovereign))
	sentinelRoot := types.ReduceToField([]byte{0x01})
	require.NoError(t, k.LastAnchoredRoot.Set(ctx, sentinelRoot))

	require.NoError(t, sovereignty.CommitFSMTransition(ctx, k, lockedExt(types.StateRecovering)))

	root, err := k.LastAnchoredRoot.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, sentinelRoot, root, "rt_last must stay pinned to the interval's actual start")

	header, err := k.HeaderHistory.Get(ctx, 101)
	require.NoError(t, err)
	require.Equal(t, types.StateRecovering, header.FsmState)
	require.True(t, header.WithdrawalLocked)
	require.Equal(t, types.ReduceToField([]byte{0xDE, 0xAD}), header.StateRoot)
}

// TestCommitFSMTransition_HeaderHistory_AccumulatesAcrossMultipleBlocks drives
// 3 real blocks against one keeper/store and confirms every height's header survives.
func TestCommitFSMTransition_HeaderHistory_AccumulatesAcrossMultipleBlocks(t *testing.T) {
	storeService, rawCtx := colltest.MockStore()
	k := keeper.NewKeeper(storeService, nil)
	sdkCtxAt := func(height int64, appHash []byte) sdk.Context {
		return sdk.NewContext(nil, cmtproto.Header{Height: height, AppHash: appHash}, false, log.NewNopLogger()).WithContext(rawCtx)
	}

	require.NoError(t, sovereignty.CommitFSMTransition(sdkCtxAt(10, []byte{0x10}), k, lockedExt(types.StateSovereign)))
	require.NoError(t, sovereignty.CommitFSMTransition(sdkCtxAt(11, []byte{0x11}), k, lockedExt(types.StateSovereign)))
	require.NoError(t, sovereignty.CommitFSMTransition(sdkCtxAt(12, []byte{0x12}), k, lockedExt(types.StateRecovering)))

	readCtx := sdkCtxAt(12, []byte{0x12}) // any height works for reads -- same underlying store
	for h, wantState := range map[uint64]string{10: types.StateSovereign, 11: types.StateSovereign, 12: types.StateRecovering} {
		header, err := k.HeaderHistory.Get(readCtx, h)
		require.NoError(t, err, "height %d must have a surviving header", h)
		require.Equal(t, wantState, header.FsmState)
	}

	// rt_last must still reflect height 10, the interval's actual first block.
	root, err := k.LastAnchoredRoot.Get(readCtx)
	require.NoError(t, err)
	require.Equal(t, types.ReduceToField([]byte{0x10}), root)
}

// TestCommitFSMTransition_HeaderHistory_PrunedOnReturnToAnchored confirms
// RECOVERING -> ANCHORED clears every HeaderHistory entry from the closed interval.
func TestCommitFSMTransition_HeaderHistory_PrunedOnReturnToAnchored(t *testing.T) {
	k, ctx := newTestKeeperCtxAt(t, 50, []byte{0x50})
	require.NoError(t, k.HeaderHistory.Set(ctx, 48, types.RecoveryHeader{FsmState: types.StateSovereign, WithdrawalLocked: true, StateRoot: []byte("h48")}))
	require.NoError(t, k.HeaderHistory.Set(ctx, 49, types.RecoveryHeader{FsmState: types.StateRecovering, WithdrawalLocked: true, StateRoot: []byte("h49")}))
	require.NoError(t, k.FSMState.Set(ctx, types.StateRecovering))

	require.NoError(t, sovereignty.CommitFSMTransition(ctx, k, lockedExt(types.StateAnchored)))

	iter, err := k.HeaderHistory.Iterate(ctx, nil)
	require.NoError(t, err)
	defer iter.Close()
	keys, err := iter.Keys()
	require.NoError(t, err)
	require.Empty(t, keys, "HeaderHistory must be fully pruned once the interval closes")
}

// TestCommitFSMTransition_HeaderHistory_NoWriteWhileStayingAnchored confirms
// ANCHORED -> ANCHORED never touches HeaderHistory or LastAnchoredRoot.
func TestCommitFSMTransition_HeaderHistory_NoWriteWhileStayingAnchored(t *testing.T) {
	k, ctx := newTestKeeperCtxAt(t, 5, []byte{0x05})

	require.NoError(t, sovereignty.CommitFSMTransition(ctx, k, lockedExt(types.StateAnchored)))

	_, err := k.HeaderHistory.Get(ctx, 5)
	require.Error(t, err, "no header should be written while staying ANCHORED")
	_, err = k.LastAnchoredRoot.Get(ctx)
	require.Error(t, err, "LastAnchoredRoot must stay unset while staying ANCHORED")
}
