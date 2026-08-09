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

	// 1. Lấy dữ liệu ngoại vi hiện tại từ Storage (đã đồng thuận qua các block trước)
	metrics, err := k.Metrics.Get(ctx)
	if err != nil {
		// Nếu chưa có dữ liệu, coi như healthy (khớp FSMInit trong spec: mọi gap = 0)
		metrics = &types.PeripheralMetrics{}
	}

	currState, err := k.FSMState.Get(ctx)
	if err != nil {
		currState = types.StateAnchored // Default State
	}

	safeBlocks, err := k.SafeBlocks.Get(ctx)
	if err != nil {
		safeBlocks = 0
	}
	suspiciousDuration, err := k.SuspiciousDuration.Get(ctx)
	if err != nil {
		suspiciousDuration = 0
	}
	proofValid, err := k.ReanchoringProofValid.Get(ctx)
	if err != nil {
		proofValid = false
	}

	// 2. Tính toán trạng thái tiếp theo dựa trên logic FSM (TLA+ Refinement)
	in := keeper.FSMInput{
		Metrics:               metrics,
		SafeBlocks:            safeBlocks,
		SuspiciousDuration:    suspiciousDuration,
		ReanchoringProofValid: proofValid,
	}
	nextState := keeper.CalculateNextState(currState, in, k.Params)

	// 3. Nếu có sự thay đổi trạng thái, thực hiện cập nhật và emit event
	if nextState != currState {
		if err := k.FSMState.Set(ctx, nextState); err != nil {
			return err
		}

		// Emit event để log lại timeline thực nghiệm (RQ4)
		sdkCtx.EventManager().EmitEvent(
			sdk.NewEvent(
				types.EventTypeFSMTransition,
				sdk.NewAttribute(types.AttributeKeyOldState, currState),
				sdk.NewAttribute(types.AttributeKeyNewState, nextState),
				sdk.NewAttribute(types.AttributeKeyBTCGap, fmt.Sprintf("%d", metrics.BtcGap)),
			),
		)

		// Log ra console để tiện theo dõi lúc chạy test
		sdkCtx.Logger().Info("Engram FSM Transition",
			"from", currState,
			"to", nextState,
			"height", sdkCtx.BlockHeight(),
		)
	}

	// 4. Cập nhật hai bộ đếm hysteresis/gray-failure-timeout (spec/core/EngramFSM.tla:337-346)
	if err := k.SafeBlocks.Set(ctx, keeper.NextSafeBlocks(currState, nextState, safeBlocks, k.Params)); err != nil {
		return err
	}
	if err := k.SuspiciousDuration.Set(ctx, keeper.NextSuspiciousDuration(currState, nextState, suspiciousDuration, k.Params)); err != nil {
		return err
	}

	return nil
}
