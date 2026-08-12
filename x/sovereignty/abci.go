package sovereignty

import (
	"context"
	"fmt"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func BeginBlocker(ctx context.Context, k *keeper.Keeper) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// 1. Read current peripheral metrics (agreed upon in a prior block).
	metrics, err := k.Metrics.Get(ctx)
	if err != nil {
		metrics = &types.PeripheralMetrics{} // no data yet: treat as healthy (FSMInit's all-gaps-zero)
	}

	currState, err := k.FSMState.Get(ctx)
	if err != nil {
		currState = types.StateAnchored
	}

	safeBlocks, err := k.SafeBlocks.Get(ctx)
	if err != nil {
		safeBlocks = 0
	}
	suspiciousDuration, err := k.SuspiciousDuration.Get(ctx)
	if err != nil {
		suspiciousDuration = 0
	}
	suspiciousSafeBlocks, err := k.SuspiciousSafeBlocks.Get(ctx)
	if err != nil {
		suspiciousSafeBlocks = 0
	}
	unhealthyStreak, err := k.UnhealthyStreak.Get(ctx)
	if err != nil {
		unhealthyStreak = 0
	}
	failedRecoveryAttempts, err := k.FailedRecoveryAttempts.Get(ctx)
	if err != nil {
		failedRecoveryAttempts = 0
	}
	proofValid, err := k.ReanchoringProofValid.Get(ctx)
	if err != nil {
		proofValid = false
	}

	// 2. Compute the next FSM state (spec/core/EngramFSM.tla's CalculateNextFSMState).
	in := keeper.FSMInput{
		Metrics:                metrics,
		SafeBlocks:             safeBlocks,
		SuspiciousDuration:     suspiciousDuration,
		SuspiciousSafeBlocks:   suspiciousSafeBlocks,
		UnhealthyStreak:        unhealthyStreak,
		FailedRecoveryAttempts: failedRecoveryAttempts,
		ReanchoringProofValid:  proofValid,
	}
	nextState := keeper.CalculateNextState(currState, in, k.Params)

	// 3. On a state change, persist it and emit an event/log for the timeline.
	if nextState != currState {
		if err := k.FSMState.Set(ctx, nextState); err != nil {
			return err
		}

		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeFSMTransition,
				sdk.NewAttribute(types.AttributeKeyOldState, currState),
				sdk.NewAttribute(types.AttributeKeyNewState, nextState),
				sdk.NewAttribute(types.AttributeKeyBTCGap, fmt.Sprintf("%d", metrics.BtcGap)),
			),
		)

		sdkCtx.Logger().Info("Engram FSM Transition",
			"from", currState,
			"to", nextState,
			"height", sdkCtx.BlockHeight(),
		)
	}

	// 4. Update the hysteresis/gray-failure-timeout/down-hysteresis counters
	// (spec/core/EngramFSM.tla:363-394).
	healthy := types.IsHealthyCondition(metrics, k.Params)
	if err := k.SafeBlocks.Set(ctx, keeper.NextSafeBlocks(currState, nextState, safeBlocks, healthy, k.Params)); err != nil {
		return err
	}
	if err := k.SuspiciousDuration.Set(ctx, keeper.NextSuspiciousDuration(currState, nextState, suspiciousDuration, k.Params)); err != nil {
		return err
	}
	if err := k.UnhealthyStreak.Set(ctx, keeper.NextUnhealthyStreak(currState, nextState, unhealthyStreak, healthy)); err != nil {
		return err
	}
	if err := k.SuspiciousSafeBlocks.Set(ctx, keeper.NextSuspiciousSafeBlocks(currState, nextState, suspiciousSafeBlocks, healthy, k.Params)); err != nil {
		return err
	}
	if err := k.FailedRecoveryAttempts.Set(ctx, keeper.NextFailedRecoveryAttempts(currState, nextState, failedRecoveryAttempts, k.Params)); err != nil {
		return err
	}

	return nil
}
