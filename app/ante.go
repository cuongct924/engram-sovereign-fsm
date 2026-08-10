package app

import (
	"errors"

	sovereigntykeeper "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	fsmtypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CircuitBreakerDecorator is an AnteDecorator that blocks high-risk
// transactions while the FSM is SOVEREIGN or RECOVERING.
type CircuitBreakerDecorator struct {
	fsmKeeper *sovereigntykeeper.Keeper
}

func NewCircuitBreakerDecorator(fk *sovereigntykeeper.Keeper) CircuitBreakerDecorator {
	return CircuitBreakerDecorator{
		fsmKeeper: fk,
	}
}

func (cbd CircuitBreakerDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	currentState, stateErr := cbd.fsmKeeper.FSMState.Get(ctx)
	if stateErr != nil {
		currentState = fsmtypes.StateAnchored
	}

	// WithdrawLocked mirrors spec/core/EngramFSM.tla's WithdrawLocked --
	// RECOVERING is included, not just SOVEREIGN, since the re-anchoring
	// proof isn't finalized until safe_blocks reaches HYSTERESIS_WAIT.
	if fsmtypes.WithdrawLocked(currentState) {
		for _, msg := range tx.GetMsgs() {
			if isHighRiskTransaction(msg) {
				return ctx, errors.New("CIRCUIT BREAKER ACTIVE: withdrawals and high-value transactions are halted while fsm_state is SOVEREIGN or RECOVERING")
			}
		}
	}

	return next(ctx, tx, simulate)
}

// isHighRiskTransaction identifies withdrawal/cross-chain transfer message
// types this app currently mounts (bank/IBC not wired -- see app.go's TODO).
func isHighRiskTransaction(msg sdk.Msg) bool {
	msgType := sdk.MsgTypeURL(msg)
	return msgType == "/cosmos.bank.v1beta1.MsgSend" || msgType == "/ibc.applications.transfer.v1.MsgTransfer"
}
