package keeper

import (
	"testing"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
)

func TestQueryState(t *testing.T) {
	k, ctx := newTestKeeper(t)

	require.NoError(t, k.FSMState.Set(ctx, types.StateSovereign))
	require.NoError(t, k.SafeBlocks.Set(ctx, 7))
	require.NoError(t, k.SuspiciousDuration.Set(ctx, 3))
	require.NoError(t, k.ReanchoringProofValid.Set(ctx, true))
	metrics := &types.PeripheralMetrics{CleanPeers: 4}
	require.NoError(t, k.Metrics.Set(ctx, metrics))

	srv := NewQueryServerImpl(k)
	resp, err := srv.State(ctx, &types.QueryStateRequest{})
	require.NoError(t, err)
	require.Equal(t, types.StateSovereign, resp.FsmState)
	require.Equal(t, uint64(7), resp.SafeBlocks)
	require.Equal(t, uint64(3), resp.SuspiciousDuration)
	require.True(t, resp.ReanchoringProofValid)
	require.Equal(t, metrics, resp.Metrics)
}

func TestQueryRecoveryHeaders(t *testing.T) {
	k, ctx := newTestKeeper(t)

	rt := types.ReduceToField([]byte("rt"))
	require.NoError(t, k.LastAnchoredRoot.Set(ctx, rt))
	require.NoError(t, k.HeaderHistory.Set(ctx, 5, types.RecoveryHeader{FsmState: types.StateSovereign, StateRoot: []byte("h5")}))
	require.NoError(t, k.HeaderHistory.Set(ctx, 3, types.RecoveryHeader{FsmState: types.StateRecovering, WithdrawalLocked: true, StateRoot: []byte("h3")}))

	srv := NewQueryServerImpl(k)
	resp, err := srv.RecoveryHeaders(ctx, &types.QueryRecoveryHeadersRequest{})
	require.NoError(t, err)
	require.Equal(t, rt, resp.LastAnchoredRoot)

	// Headers must come back ascending by height (witness-chain order), even
	// though inserted out of order above.
	require.Len(t, resp.Headers, 2)
	require.Equal(t, uint64(3), resp.Headers[0].Height)
	require.Equal(t, uint64(5), resp.Headers[1].Height)
	require.Equal(t, types.StateRecovering, resp.Headers[0].FsmState)
	require.True(t, resp.Headers[0].WithdrawalLocked)
	require.Equal(t, []byte("h3"), resp.Headers[0].StateRoot)
}

func TestQueryRecoveryHeadersEmpty(t *testing.T) {
	k, ctx := newTestKeeper(t)
	srv := NewQueryServerImpl(k)
	resp, err := srv.RecoveryHeaders(ctx, &types.QueryRecoveryHeadersRequest{})
	require.NoError(t, err)
	require.Nil(t, resp.LastAnchoredRoot)
	require.Empty(t, resp.Headers)
}
