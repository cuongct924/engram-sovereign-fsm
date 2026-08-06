package keeper

import (
	"context"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// QueryServerImpl implements types.QueryServer. Previously this module had
// no QueryServer at all -- Query.State was defined in the proto and had
// generated types, but was never implemented or registered (module.go's
// RegisterServices only ever called types.RegisterMsgServer), so it was
// unreachable via gRPC/CLI despite existing on paper.
type QueryServerImpl struct {
	*Keeper
}

func NewQueryServerImpl(k *Keeper) types.QueryServer {
	return &QueryServerImpl{Keeper: k}
}

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

// RecoveryHeaders dumps HeaderHistory + LastAnchoredRoot exactly as tracked
// on-chain, so scripts/reanchoring_prover.sh (A6) can build a real ZK
// witness without independently reconstructing this validator's view of the
// current SOVEREIGN/RECOVERING interval.
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

	// kvs is already ascending by height (collections.Map's default
	// iteration order, backed by the KVStore's own sorted key iteration) --
	// exactly the order a witness chain must be assembled in.
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
