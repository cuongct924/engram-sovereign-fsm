package keeper

import (
	"context"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// QueryServerImpl implements types.QueryServer (Query.State + RecoveryHeaders).
type QueryServerImpl struct {
	*Keeper
}

func NewQueryServerImpl(k *Keeper) types.QueryServer {
	return &QueryServerImpl{Keeper: k}
}

// State dumps the node's current FSM snapshot -- serves docs/EXPERIMENT.md's
// E9 trace-driven stress test and E2 observation (not part of consensus).
func (k *QueryServerImpl) State(ctx context.Context, _ *types.QueryStateRequest) (*types.QueryStateResponse, error) {
	fsmState, _ := k.FSMState.Get(ctx)
	safeBlocks, _ := k.SafeBlocks.Get(ctx)
	suspiciousDuration, _ := k.SuspiciousDuration.Get(ctx)
	proofValid, _ := k.ReanchoringProofValid.Get(ctx)
	metrics, _ := k.Metrics.Get(ctx)

	return &types.QueryStateResponse{
		FsmState:              fsmState,
		SafeBlocks:            safeBlocks,
		SuspiciousDuration:    suspiciousDuration,
		ReanchoringProofValid: proofValid,
		Metrics:               metrics,
	}, nil
}

// RecoveryHeaders dumps the current interval's HeaderHistory + rt_last so an
// off-chain prover can build a ZK witness without reconstructing the view.
func (k *QueryServerImpl) RecoveryHeaders(ctx context.Context, _ *types.QueryRecoveryHeadersRequest) (*types.QueryRecoveryHeadersResponse, error) {
	lastAnchoredRoot, _ := k.LastAnchoredRoot.Get(ctx)

	iter, err := k.HeaderHistory.Iterate(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	kvs, err := iter.KeyValues()
	if err != nil {
		return nil, err
	}

	// Already ascending by height (KVStore sorted keys) -- the order a witness
	// chain needs.
	headers := make([]*types.QueryRecoveryHeader, 0, len(kvs))
	for _, kv := range kvs {
		headers = append(headers, &types.QueryRecoveryHeader{
			Height:           kv.Key,
			FsmState:         kv.Value.FsmState,
			WithdrawalLocked: kv.Value.WithdrawalLocked,
			StateRoot:        kv.Value.StateRoot,
		})
	}

	return &types.QueryRecoveryHeadersResponse{
		LastAnchoredRoot: lastAnchoredRoot,
		Headers:          headers,
	}, nil
}
