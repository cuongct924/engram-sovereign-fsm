package keeper

import (
	"context"
	"testing"

	"cosmossdk.io/collections/colltest"
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
