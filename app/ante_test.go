package app

import (
	"fmt"
	"testing"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sovereigntykeeper "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	fsmtypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	"cosmossdk.io/collections/colltest"
	log "cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// stubTx is a minimal sdk.Tx carrying a fixed message list -- enough for
// CircuitBreakerDecorator.AnteHandle, which only calls tx.GetMsgs().
type stubTx struct {
	msgs []sdk.Msg
}

func (s stubTx) GetMsgs() []sdk.Msg                    { return s.msgs }
func (s stubTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

// newTestAnteFixture wires a real Keeper (colltest.MockStore pattern) into an
// sdk.Context whose underlying context.Context is the mock store's, so
// collections reads/writes resolve.
func newTestAnteFixture(t *testing.T) (CircuitBreakerDecorator, sdk.Context, *sovereigntykeeper.Keeper) {
	t.Helper()
	storeService, mockCtx := colltest.MockStore()
	k := sovereigntykeeper.NewKeeper(storeService, nil)
	sdkCtx := sdk.NewContext(nil, cmtproto.Header{}, false, log.NewNopLogger()).WithContext(mockCtx)
	return NewCircuitBreakerDecorator(k), sdkCtx, k
}

func callNext(nextCalled *bool) sdk.AnteHandler {
	return func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		*nextCalled = true
		return ctx, nil
	}
}

// TestAnteHandle_NoPersistedStateDefaultsToAnchored mirrors AnteHandle's
// fallback: FSMState.Get erroring (nothing persisted, e.g. genesis) must
// default to ANCHORED, not block anything.
func TestAnteHandle_NoPersistedStateDefaultsToAnchored(t *testing.T) {
	cbd, ctx, _ := newTestAnteFixture(t)
	tx := stubTx{msgs: []sdk.Msg{&banktypes.MsgSend{}}}

	var nextCalled bool
	_, err := cbd.AnteHandle(ctx, tx, false, callNext(&nextCalled))
	require.NoError(t, err)
	require.True(t, nextCalled, "ANCHORED default must not block a withdrawal")
}

// TestAnteHandle_BlocksHighRiskTxWhileSovereign mirrors WithdrawLocked in
// spec/core/EngramFSM.tla: withdrawals must halt while fsm_state is SOVEREIGN.
func TestAnteHandle_BlocksHighRiskTxWhileSovereign(t *testing.T) {
	cbd, ctx, k := newTestAnteFixture(t)
	require.NoError(t, k.FSMState.Set(ctx, fsmtypes.StateSovereign))
	tx := stubTx{msgs: []sdk.Msg{&banktypes.MsgSend{}}}

	var nextCalled bool
	_, err := cbd.AnteHandle(ctx, tx, false, callNext(&nextCalled))
	require.Error(t, err, "MsgSend must be blocked while SOVEREIGN")
	require.False(t, nextCalled)
}

// TestAnteHandle_BlocksHighRiskTxWhileRecovering covers the RECOVERING half
// of WithdrawLocked -- re-anchoring isn't finalized until safe_blocks reaches
// HysteresisWait, so withdrawals stay halted here too, not just in SOVEREIGN.
func TestAnteHandle_BlocksHighRiskTxWhileRecovering(t *testing.T) {
	cbd, ctx, k := newTestAnteFixture(t)
	require.NoError(t, k.FSMState.Set(ctx, fsmtypes.StateRecovering))
	tx := stubTx{msgs: []sdk.Msg{&banktypes.MsgSend{}}}

	var nextCalled bool
	_, err := cbd.AnteHandle(ctx, tx, false, callNext(&nextCalled))
	require.Error(t, err, "MsgSend must be blocked while RECOVERING")
	require.False(t, nextCalled)
}

// TestAnteHandle_AllowsHighRiskTxWhileAnchoredOrSuspicious confirms the
// breaker is scoped to WithdrawLocked's exact two states, not a broader
// "anything but healthy" condition.
func TestAnteHandle_AllowsHighRiskTxWhileAnchoredOrSuspicious(t *testing.T) {
	for _, state := range []string{fsmtypes.StateAnchored, fsmtypes.StateSuspicious} {
		t.Run(state, func(t *testing.T) {
			cbd, ctx, k := newTestAnteFixture(t)
			require.NoError(t, k.FSMState.Set(ctx, state))
			tx := stubTx{msgs: []sdk.Msg{&banktypes.MsgSend{}}}

			var nextCalled bool
			_, err := cbd.AnteHandle(ctx, tx, false, callNext(&nextCalled))
			require.NoError(t, err)
			require.True(t, nextCalled)
		})
	}
}

// TestAnteHandle_AllowsNonWithdrawalTxWhileSovereign confirms the breaker
// only targets isHighRiskTransaction's specific message types, not every
// transaction, while SOVEREIGN.
func TestAnteHandle_AllowsNonWithdrawalTxWhileSovereign(t *testing.T) {
	cbd, ctx, k := newTestAnteFixture(t)
	require.NoError(t, k.FSMState.Set(ctx, fsmtypes.StateSovereign))
	tx := stubTx{msgs: []sdk.Msg{&fsmtypes.MsgInjectFaultRequest{}}}

	var nextCalled bool
	_, err := cbd.AnteHandle(ctx, tx, false, callNext(&nextCalled))
	require.NoError(t, err)
	require.True(t, nextCalled)
}

func TestIsHighRiskTransaction(t *testing.T) {
	require.True(t, isHighRiskTransaction(&banktypes.MsgSend{}))
	require.False(t, isHighRiskTransaction(&fsmtypes.MsgInjectFaultRequest{}))
}

// seedForcedTxQueue queues n distinct forced txs, for tests exercising the
// SUSPICIOUS admission cap against a specific starting queue size.
func seedForcedTxQueue(t *testing.T, ctx sdk.Context, k *sovereigntykeeper.Keeper, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		require.NoError(t, k.ForcedTxQueue.Set(ctx, fmt.Sprintf("seed-tx-%d", i)))
	}
}

func TestAnteHandle_AllowsForcedTxWhileSuspiciousUnderCap(t *testing.T) {
	cbd, ctx, k := newTestAnteFixture(t)
	require.NoError(t, k.FSMState.Set(ctx, fsmtypes.StateSuspicious))
	k.Params.MaxSuspiciousForcedTxQueue = 3
	seedForcedTxQueue(t, ctx, k, 2) // under cap

	tx := stubTx{msgs: []sdk.Msg{&fsmtypes.MsgSubmitForcedTxRequest{}}}
	var nextCalled bool
	_, err := cbd.AnteHandle(ctx, tx, false, callNext(&nextCalled))
	require.NoError(t, err)
	require.True(t, nextCalled)
}

func TestAnteHandle_BlocksForcedTxWhileSuspiciousAtCap(t *testing.T) {
	cbd, ctx, k := newTestAnteFixture(t)
	require.NoError(t, k.FSMState.Set(ctx, fsmtypes.StateSuspicious))
	k.Params.MaxSuspiciousForcedTxQueue = 3
	seedForcedTxQueue(t, ctx, k, 3) // exactly at cap

	tx := stubTx{msgs: []sdk.Msg{&fsmtypes.MsgSubmitForcedTxRequest{}}}
	var nextCalled bool
	_, err := cbd.AnteHandle(ctx, tx, false, callNext(&nextCalled))
	require.Error(t, err, "must reject once the queue is already at cap")
	require.False(t, nextCalled)
}

// TestAnteHandle_ForcedTxThrottleOnlyAppliesToSuspicious confirms the new
// check never fires outside SUSPICIOUS, even at/over cap.
func TestAnteHandle_ForcedTxThrottleOnlyAppliesToSuspicious(t *testing.T) {
	for _, state := range []string{fsmtypes.StateAnchored, fsmtypes.StateSovereign, fsmtypes.StateRecovering} {
		t.Run(state, func(t *testing.T) {
			cbd, ctx, k := newTestAnteFixture(t)
			require.NoError(t, k.FSMState.Set(ctx, state))
			k.Params.MaxSuspiciousForcedTxQueue = 3
			seedForcedTxQueue(t, ctx, k, 5) // over cap

			tx := stubTx{msgs: []sdk.Msg{&fsmtypes.MsgSubmitForcedTxRequest{}}}
			var nextCalled bool
			_, err := cbd.AnteHandle(ctx, tx, false, callNext(&nextCalled))
			require.NoError(t, err, "the SUSPICIOUS-only throttle must never fire outside SUSPICIOUS")
			require.True(t, nextCalled)
		})
	}
}

func TestAnteHandle_NeverThrottlesRecoveryProofWhileSuspiciousAtCap(t *testing.T) {
	cbd, ctx, k := newTestAnteFixture(t)
	require.NoError(t, k.FSMState.Set(ctx, fsmtypes.StateSuspicious))
	k.Params.MaxSuspiciousForcedTxQueue = 3
	seedForcedTxQueue(t, ctx, k, 5) // over cap

	tx := stubTx{msgs: []sdk.Msg{&fsmtypes.MsgSubmitRecoveryProofRequest{}}}
	var nextCalled bool
	_, err := cbd.AnteHandle(ctx, tx, false, callNext(&nextCalled))
	require.NoError(t, err, "the only escape from SOVEREIGN/RECOVERING must never be throttled")
	require.True(t, nextCalled)
}

func TestAnteHandle_AllowsNonForcedTxWhileSuspiciousAtCap(t *testing.T) {
	cbd, ctx, k := newTestAnteFixture(t)
	require.NoError(t, k.FSMState.Set(ctx, fsmtypes.StateSuspicious))
	k.Params.MaxSuspiciousForcedTxQueue = 3
	seedForcedTxQueue(t, ctx, k, 5) // over cap

	tx := stubTx{msgs: []sdk.Msg{&fsmtypes.MsgInjectFaultRequest{}}}
	var nextCalled bool
	_, err := cbd.AnteHandle(ctx, tx, false, callNext(&nextCalled))
	require.NoError(t, err, "the throttle must be scoped to MsgSubmitForcedTxRequest only")
	require.True(t, nextCalled)
}
