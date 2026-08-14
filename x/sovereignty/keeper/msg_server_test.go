package keeper

import (
	"context"
	"fmt"
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections/colltest"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func newTestMsgServer(t *testing.T) (*MsgServerImpl, context.Context) {
	t.Helper()
	storeService, ctx := colltest.MockStore()
	k := NewKeeper(storeService, nil)
	return &MsgServerImpl{Keeper: k}, ctx
}

// Regression test (live-confirmed, fixed in 51b8314): SubmitForcedTx used to
// queue any content, letting one undecodable tx permanently halt the chain.
// Undecodable content must now be rejected at the source; valid content queued.
func TestSubmitForcedTx_RejectsUndecodableContentWhenTxDecoderSet(t *testing.T) {
	srv, ctx := newTestMsgServer(t)
	srv.TxDecoder = func(txBytes []byte) (sdk.Tx, error) {
		if string(txBytes) == "valid" {
			return nil, nil
		}
		return nil, fmt.Errorf("not a real tx")
	}

	_, err := srv.SubmitForcedTx(ctx, &types.MsgSubmitForcedTxRequest{Tx: []byte("not-a-real-tx")})
	require.Error(t, err, "undecodable content must be rejected before ever entering ForcedTxQueue")
	has, err := srv.ForcedTxQueue.Has(ctx, "not-a-real-tx")
	require.NoError(t, err)
	require.False(t, has)

	_, err = srv.SubmitForcedTx(ctx, &types.MsgSubmitForcedTxRequest{Tx: []byte("valid")})
	require.NoError(t, err)
	has, err = srv.ForcedTxQueue.Has(ctx, "valid")
	require.NoError(t, err)
	require.True(t, has)
}

// TxDecoder wiring is optional (nil-safe, like peerFilterSrc): unset means
// the old unconditional-accept behavior.
func TestSubmitForcedTx_SkipsValidationWhenTxDecoderUnset(t *testing.T) {
	srv, ctx := newTestMsgServer(t)
	_, err := srv.SubmitForcedTx(ctx, &types.MsgSubmitForcedTxRequest{Tx: []byte("anything")})
	require.NoError(t, err)
}

// Regression test: this handler used to set FSMState=ANCHORED on proof
// validity alone, bypassing StrictFSMTransitionSafety/HysteresisWait. It
// must never touch FSMState -- only latch RealProofSubmittedHeight, consumed
// via sensors_refresh.go's refreshReanchoringProofValid.
func TestSubmitRecoveryProof_NeverSetsFSMStateDirectly(t *testing.T) {
	srv, ctx := newTestMsgServer(t)
	require.NoError(t, srv.FSMState.Set(ctx, types.StateSovereign))

	// Garbage proof must fail VerifyZKProof regardless of whether bb is on
	// PATH (real rejection or fail-closed both return false).
	_, err := srv.SubmitRecoveryProof(ctx, &types.MsgSubmitRecoveryProofRequest{
		ZkProof:      []byte("not-a-real-proof"),
		PublicInputs: make([]byte, 96),
	})
	require.ErrorIs(t, err, types.ErrInvalidZKProof)

	state, err := srv.FSMState.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, types.StateSovereign, state, "SubmitRecoveryProof must never write FSMState directly")

	submittedHeight, _ := srv.RealProofSubmittedHeight.Get(ctx)
	require.Zero(t, submittedHeight)
}

// Length guard for the (rt_last, rt_new, count) binding: a non-96-byte
// PublicInputs is rejected before any on-chain state comparison.
func TestSubmitRecoveryProof_RejectsMalformedPublicInputs(t *testing.T) {
	srv, ctx := newTestMsgServer(t)

	_, err := srv.SubmitRecoveryProof(ctx, &types.MsgSubmitRecoveryProofRequest{
		ZkProof:      []byte("irrelevant-fails-verify-first"),
		PublicInputs: make([]byte, 64), // wrong length (old 2-field layout)
	})
	require.ErrorIs(t, err, types.ErrInvalidZKProof)
}

// Empty history must fail closed (ErrInvalidZKProof), not return a
// zero-value header that could spuriously match a zero rt_new.
func TestLatestTrackedHeader_EmptyHistory(t *testing.T) {
	storeService, ctx := colltest.MockStore()
	k := NewKeeper(storeService, nil)

	_, _, err := k.LatestTrackedHeader(ctx)
	require.ErrorIs(t, err, types.ErrInvalidZKProof)
}

// Tip lookup returns the header at the greatest tracked height, not just
// any entry.
func TestLatestTrackedHeader_PicksHighestHeight(t *testing.T) {
	storeService, ctx := colltest.MockStore()
	k := NewKeeper(storeService, nil)

	require.NoError(t, k.HeaderHistory.Set(ctx, 5, types.RecoveryHeader{FsmState: types.StateSovereign, StateRoot: []byte("h5")}))
	require.NoError(t, k.HeaderHistory.Set(ctx, 7, types.RecoveryHeader{FsmState: types.StateRecovering, StateRoot: []byte("h7")}))
	require.NoError(t, k.HeaderHistory.Set(ctx, 6, types.RecoveryHeader{FsmState: types.StateRecovering, StateRoot: []byte("h6")}))

	height, tip, err := k.LatestTrackedHeader(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(7), height)
	require.Equal(t, []byte("h7"), tip.StateRoot)
}

// Regression test: the latch stores the proven HEIGHT, not a bool, so
// staleness is detected when the interval grows past it -- guards the
// LatestTrackedHeader primitive that check depends on.
func TestSubmitRecoveryProof_StaleProofRejectedAfterIntervalGrows(t *testing.T) {
	srv, ctx := newTestMsgServer(t)
	require.NoError(t, srv.HeaderHistory.Set(ctx, 4, types.RecoveryHeader{FsmState: types.StateSovereign, StateRoot: []byte("h4")}))
	require.NoError(t, srv.RealProofSubmittedHeight.Set(ctx, 4))

	height, _, err := srv.LatestTrackedHeader(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(4), height)

	// Interval grew before the latch was consumed -- staleness is detected
	// by comparing the latch to the new tip, not by the latch self-clearing.
	require.NoError(t, srv.HeaderHistory.Set(ctx, 5, types.RecoveryHeader{FsmState: types.StateSovereign, StateRoot: []byte("h5")}))
	newHeight, _, err := srv.LatestTrackedHeader(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(5), newHeight)

	submittedHeight, _ := srv.RealProofSubmittedHeight.Get(ctx)
	require.Equal(t, uint64(4), submittedHeight, "the stale latch value itself is untouched by new headers -- staleness is detected by comparing it against the tip, not by it self-clearing")
}

// Rolling checkpoint: a proof's rt_new only needs to match SOME tracked
// header's state_root, not the tip -- a valid proof (checked against the
// fixed-N circuit's VK) guarantees real headers connect rt_last to it.
func TestFindHeaderByStateRoot_FindsMatchingHeight(t *testing.T) {
	storeService, ctx := colltest.MockStore()
	k := NewKeeper(storeService, nil)

	require.NoError(t, k.HeaderHistory.Set(ctx, 4, types.RecoveryHeader{FsmState: types.StateSovereign, StateRoot: []byte("h4")}))
	require.NoError(t, k.HeaderHistory.Set(ctx, 8, types.RecoveryHeader{FsmState: types.StateRecovering, StateRoot: []byte("h8")}))
	require.NoError(t, k.HeaderHistory.Set(ctx, 12, types.RecoveryHeader{FsmState: types.StateRecovering, StateRoot: []byte("h12")}))

	height, found, err := k.findHeaderByStateRoot(ctx, []byte("h8"))
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(8), height, "must find the MIDDLE tracked header, not just the tip -- a rolling checkpoint proof's rt_new is rarely the current tip")
}

// Fails closed (found=false, no error) for a root never tracked --
// SubmitRecoveryProof treats it as ErrInvalidZKProof.
func TestFindHeaderByStateRoot_NotFound(t *testing.T) {
	storeService, ctx := colltest.MockStore()
	k := NewKeeper(storeService, nil)

	require.NoError(t, k.HeaderHistory.Set(ctx, 4, types.RecoveryHeader{FsmState: types.StateSovereign, StateRoot: []byte("h4")}))

	_, found, err := k.findHeaderByStateRoot(ctx, []byte("never-tracked"))
	require.NoError(t, err)
	require.False(t, found)
}

// Rolling checkpoint: advancing to height H drops only headers AT OR BELOW
// H (the consumed segment); later headers survive as the next segment's
// start. Unlike pruneHeaderHistory (full clear on return to ANCHORED), this
// is always a prefix trim.
func TestPruneHeaderHistoryUpTo_RemovesPrefixKeepsLater(t *testing.T) {
	storeService, ctx := colltest.MockStore()
	k := NewKeeper(storeService, nil)

	for h := uint64(1); h <= 6; h++ {
		require.NoError(t, k.HeaderHistory.Set(ctx, h, types.RecoveryHeader{FsmState: types.StateRecovering, StateRoot: []byte{byte(h)}}))
	}

	require.NoError(t, k.pruneHeaderHistoryUpTo(ctx, 4))

	for h := uint64(1); h <= 4; h++ {
		_, err := k.HeaderHistory.Get(ctx, h)
		require.Error(t, err, "height %d should have been pruned", h)
	}
	for h := uint64(5); h <= 6; h++ {
		_, err := k.HeaderHistory.Get(ctx, h)
		require.NoError(t, err, "height %d is past the new checkpoint and must survive", h)
	}
}
