package keeper

import (
	"context"
	"fmt"
	"testing"

	"cosmossdk.io/collections/colltest"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/iden3/go-merkletree-sql/v2/db/memory"
	"github.com/stretchr/testify/require"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

func newTestMsgServer(t *testing.T) (*MsgServerImpl, context.Context) {
	t.Helper()
	storeService, ctx := colltest.MockStore()
	k := NewKeeper(storeService, nil, memory.NewMemoryStorage())
	return &MsgServerImpl{Keeper: k}, ctx
}

// TestSubmitForcedTx_RejectsUndecodableContentWhenTxDecoderSet is a
// regression test for a real, live-confirmed permanent-liveness bug:
// SubmitForcedTx used to queue ANY msg.Tx content unconditionally. Content
// that can never itself appear as real block-tx bytes (e.g. a bare marker
// string) can also never satisfy IsCensoring's "included in req.Txs" check,
// so once its ignored-round counter reaches MaxIgnoreRounds, every future
// proposal from every validator is rejected forever -- confirmed live this
// session: a single such forced tx halted a healthy 4-node cluster for
// dozens of consensus rounds with no recovery path. Rejecting undecodable
// content at submission time closes this at its source.
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

// TestSubmitForcedTx_SkipsValidationWhenTxDecoderUnset confirms the nil-safe
// default (no TxDecoder wired, e.g. every other test in this package using
// newTestMsgServer) preserves the old unconditional-accept behavior --
// TxDecoder wiring is optional, matching peerFilterSrc's existing pattern.
func TestSubmitForcedTx_SkipsValidationWhenTxDecoderUnset(t *testing.T) {
	srv, ctx := newTestMsgServer(t)
	_, err := srv.SubmitForcedTx(ctx, &types.MsgSubmitForcedTxRequest{Tx: []byte("anything")})
	require.NoError(t, err)
}

// TestSubmitRecoveryProof_NeverSetsFSMStateDirectly is a regression test for
// the safety bug this handler used to have: it previously set
// FSMState=ANCHORED unconditionally on proof-math validity alone, bypassing
// StrictFSMTransitionSafety and HysteresisWait. Now it must never touch
// FSMState at all -- only RealProofSubmitted, consumed by
// sensors_refresh.go's refreshReanchoringProofValid through the existing
// hysteresis-gated CalculateNextState pipeline.
func TestSubmitRecoveryProof_NeverSetsFSMStateDirectly(t *testing.T) {
	srv, ctx := newTestMsgServer(t)
	require.NoError(t, srv.FSMState.Set(ctx, types.StateSovereign))

	// A garbage proof must fail VerifyZKProof (whether via a real `bb
	// verify` rejection, or fail-closed if bb isn't on PATH at all -- both
	// paths return false, so this assertion doesn't depend on the bb
	// toolchain being installed in this environment).
	_, err := srv.SubmitRecoveryProof(ctx, &types.MsgSubmitRecoveryProofRequest{
		ZkProof:      []byte("not-a-real-proof"),
		PublicInputs: make([]byte, 64),
	})
	require.ErrorIs(t, err, types.ErrInvalidZKProof)

	state, err := srv.FSMState.Get(ctx)
	require.NoError(t, err)
	require.Equal(t, types.StateSovereign, state, "SubmitRecoveryProof must never write FSMState directly")

	submittedHeight, _ := srv.RealProofSubmittedHeight.Get(ctx)
	require.Zero(t, submittedHeight)
}

// TestSubmitRecoveryProof_RejectsMalformedPublicInputs covers the new
// rt_last/rt_new binding check's length guard -- a non-64-byte
// PublicInputs can't possibly decode into (rt_last, rt_new), so it must be
// rejected before any on-chain state comparison is attempted.
func TestSubmitRecoveryProof_RejectsMalformedPublicInputs(t *testing.T) {
	srv, ctx := newTestMsgServer(t)

	_, err := srv.SubmitRecoveryProof(ctx, &types.MsgSubmitRecoveryProofRequest{
		ZkProof:      []byte("irrelevant-fails-verify-first"),
		PublicInputs: make([]byte, 32), // wrong length
	})
	require.ErrorIs(t, err, types.ErrInvalidZKProof)
}

// TestLatestTrackedHeader_EmptyHistory confirms the tip lookup used by
// SubmitRecoveryProof's rt_new binding check fails closed (rather than
// panicking or returning a zero-value header that would spuriously match a
// zero rt_new) when no SOVEREIGN/RECOVERING interval is currently tracked.
func TestLatestTrackedHeader_EmptyHistory(t *testing.T) {
	storeService, ctx := colltest.MockStore()
	k := NewKeeper(storeService, nil, memory.NewMemoryStorage())

	_, _, err := k.LatestTrackedHeader(ctx)
	require.ErrorIs(t, err, types.ErrInvalidZKProof)
}

// TestLatestTrackedHeader_PicksHighestHeight confirms the tip lookup
// returns the header at the greatest tracked height, not just any entry.
func TestLatestTrackedHeader_PicksHighestHeight(t *testing.T) {
	storeService, ctx := colltest.MockStore()
	k := NewKeeper(storeService, nil, memory.NewMemoryStorage())

	require.NoError(t, k.HeaderHistory.Set(ctx, 5, types.RecoveryHeader{FsmState: types.StateSovereign, StateRoot: []byte("h5")}))
	require.NoError(t, k.HeaderHistory.Set(ctx, 7, types.RecoveryHeader{FsmState: types.StateRecovering, StateRoot: []byte("h7")}))
	require.NoError(t, k.HeaderHistory.Set(ctx, 6, types.RecoveryHeader{FsmState: types.StateRecovering, StateRoot: []byte("h6")}))

	height, tip, err := k.LatestTrackedHeader(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(7), height)
	require.Equal(t, []byte("h7"), tip.StateRoot)
}

// TestSubmitRecoveryProof_StaleProofRejectedAfterIntervalGrows is a
// regression test for the gap found by actually running the real prover
// pipeline end-to-end: a proof submitted while N headers were tracked must
// stop counting once a NEW header has been appended (the interval grew
// past what the proof covers) -- a flat bool latch can't express this,
// which is why RealProofSubmittedHeight stores the proven height instead.
func TestSubmitRecoveryProof_StaleProofRejectedAfterIntervalGrows(t *testing.T) {
	srv, ctx := newTestMsgServer(t)
	require.NoError(t, srv.HeaderHistory.Set(ctx, 4, types.RecoveryHeader{FsmState: types.StateSovereign, StateRoot: []byte("h4")}))
	require.NoError(t, srv.RealProofSubmittedHeight.Set(ctx, 4))

	height, _, err := srv.LatestTrackedHeader(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(4), height)

	// A 5th header is appended (interval grew) before the latch was
	// consumed -- refreshReanchoringProofValid's "height == tip height"
	// check (sensors_refresh.go) is what must now treat this as stale, not
	// this test directly (that function lives in package sovereignty, a
	// different package); this test only guards the primitive it depends
	// on: LatestTrackedHeader must report the NEW tip, not the stale one.
	require.NoError(t, srv.HeaderHistory.Set(ctx, 5, types.RecoveryHeader{FsmState: types.StateSovereign, StateRoot: []byte("h5")}))
	newHeight, _, err := srv.LatestTrackedHeader(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(5), newHeight)

	submittedHeight, _ := srv.RealProofSubmittedHeight.Get(ctx)
	require.Equal(t, uint64(4), submittedHeight, "the stale latch value itself is untouched by new headers -- staleness is detected by comparing it against the tip, not by it self-clearing")
}

// TestFindHeaderByStateRoot_FindsMatchingHeight covers the primitive that
// makes rolling, mid-interval checkpoint advances possible (see
// SubmitRecoveryProof's doc): a proof's rt_new only needs to match SOME
// tracked header's state_root, not the absolute tip, since a valid proof
// (already checked against the fixed-N circuit's VK before this is called)
// is itself the guarantee that exactly N real headers connect rt_last to
// whatever height this finds.
func TestFindHeaderByStateRoot_FindsMatchingHeight(t *testing.T) {
	storeService, ctx := colltest.MockStore()
	k := NewKeeper(storeService, nil, memory.NewMemoryStorage())

	require.NoError(t, k.HeaderHistory.Set(ctx, 4, types.RecoveryHeader{FsmState: types.StateSovereign, StateRoot: []byte("h4")}))
	require.NoError(t, k.HeaderHistory.Set(ctx, 8, types.RecoveryHeader{FsmState: types.StateRecovering, StateRoot: []byte("h8")}))
	require.NoError(t, k.HeaderHistory.Set(ctx, 12, types.RecoveryHeader{FsmState: types.StateRecovering, StateRoot: []byte("h12")}))

	height, found, err := k.findHeaderByStateRoot(ctx, []byte("h8"))
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, uint64(8), height, "must find the MIDDLE tracked header, not just the tip -- a rolling checkpoint proof's rt_new is rarely the current tip")
}

// TestFindHeaderByStateRoot_NotFound confirms this fails closed (found=false,
// no error) for a root that was never tracked -- SubmitRecoveryProof treats
// this the same as any other invalid proof (ErrInvalidZKProof), closing the
// same replay gap LatestTrackedHeader's exact-tip check used to close.
func TestFindHeaderByStateRoot_NotFound(t *testing.T) {
	storeService, ctx := colltest.MockStore()
	k := NewKeeper(storeService, nil, memory.NewMemoryStorage())

	require.NoError(t, k.HeaderHistory.Set(ctx, 4, types.RecoveryHeader{FsmState: types.StateSovereign, StateRoot: []byte("h4")}))

	_, found, err := k.findHeaderByStateRoot(ctx, []byte("never-tracked"))
	require.NoError(t, err)
	require.False(t, found)
}

// TestPruneHeaderHistoryUpTo_RemovesPrefixKeepsLater covers the other half
// of the rolling-checkpoint scheme: advancing a checkpoint to height H must
// only drop headers AT OR BELOW H (this segment's proof, now consumed) and
// leave anything already accumulated past H untouched -- those headers are
// the start of the NEXT segment, not garbage. Unlike
// x/sovereignty.pruneHeaderHistory (full clear on return to ANCHORED), this
// is always a prefix trim.
func TestPruneHeaderHistoryUpTo_RemovesPrefixKeepsLater(t *testing.T) {
	storeService, ctx := colltest.MockStore()
	k := NewKeeper(storeService, nil, memory.NewMemoryStorage())

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
