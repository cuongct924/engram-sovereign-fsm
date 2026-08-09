package app

import (
	"errors"

	sovereigntykeeper "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	fsmtypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CircuitBreakerDecorator is an AnteDecorator that runs before transactions are added to the mempool
type CircuitBreakerDecorator struct {
	fsmKeeper *sovereigntykeeper.Keeper
}

func NewCircuitBreakerDecorator(fk *sovereigntykeeper.Keeper) CircuitBreakerDecorator {
	return CircuitBreakerDecorator{
		fsmKeeper: fk,
	}
}

// AnteHandle implements the Circuit Breaker logic based on the current FSM state
func (cbd CircuitBreakerDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	// 1. Retrieve the current FSM state from the Keeper
	currentState, stateErr := cbd.fsmKeeper.FSMState.Get(ctx)
	if stateErr != nil {
		currentState = fsmtypes.StateAnchored // no state persisted yet => default
	}

	// 2. Withdrawals / high-risk transactions are halted whenever the network
	// is SOVEREIGN or RECOVERING (fsmtypes.WithdrawLocked mirrors WithdrawLocked
	// in spec/core/EngramFSM.tla; RECOVERING must be included, not just SOVEREIGN,
	// since re-anchoring proof isn't finalized until safe_blocks reaches HYSTERESIS_WAIT).
	if fsmtypes.WithdrawLocked(currentState) {
		for _, msg := range tx.GetMsgs() {
			// ACTIVATE CIRCUIT BREAKER: Block all withdrawal transactions or cross-chain asset transfers (IBC Transfer)
			if isHighRiskTransaction(msg) {
				return ctx, errors.New("CIRCUIT BREAKER ACTIVE: withdrawals and high-value transactions are halted while fsm_state is SOVEREIGN or RECOVERING")
			}
		}
	}

	// If the transaction is safe or the network is in ANCHORED/SUSPICIOUS state, allow it to proceed
	return next(ctx, tx, simulate)
}

// isHighRiskTransaction is a helper function to check the type of transaction
func isHighRiskTransaction(msg sdk.Msg) bool {
	// In practice, you would map this to types like banktypes.MsgSend, ibctypes.MsgTransfer...
	msgType := sdk.MsgTypeURL(msg)
	if msgType == "/cosmos.bank.v1beta1.MsgSend" || msgType == "/ibc.applications.transfer.v1.MsgTransfer" {
		return true
	}
	return false
}
