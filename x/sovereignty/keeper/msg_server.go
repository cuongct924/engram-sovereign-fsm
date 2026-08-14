package keeper

import (
	"bytes"
	"context"
	"fmt"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

type MsgServerImpl struct {
	*Keeper
}

func NewMsgServerImpl(k *Keeper) types.MsgServer {
	return &MsgServerImpl{Keeper: k}
}

// InjectFault overwrites PeripheralMetrics directly, for tests/experiments.
func (k *MsgServerImpl) InjectFault(ctx context.Context, msg *types.MsgInjectFaultRequest) (*types.MsgInjectFaultResponse, error) {
	if msg.FaultInputs != nil {
		if err := k.Metrics.Set(ctx, msg.FaultInputs); err != nil {
			return nil, err
		}
	}
	return &types.MsgInjectFaultResponse{}, nil
}

// SubmitForcedTx ports SubmitToCelestiaDA (spec/core/EngramTendermint.tla:886-892):
// queues tx into forced_tx_queue, enforced by IsCensoring. Rejects content
// that won't decode as a real tx -- undecodable bytes could never satisfy
// IsCensoring's inclusion check, halting the chain permanently once the
// ignored-round counter hits MaxIgnoreRounds (fixed in 51b8314).
func (k *MsgServerImpl) SubmitForcedTx(ctx context.Context, msg *types.MsgSubmitForcedTxRequest) (*types.MsgSubmitForcedTxResponse, error) {
	if k.TxDecoder != nil {
		if _, err := k.TxDecoder(msg.Tx); err != nil {
			return nil, fmt.Errorf("forced tx content does not decode as a valid tx (would be permanently unsatisfiable): %w", err)
		}
	}
	if err := k.ForcedTxQueue.Set(ctx, string(msg.Tx)); err != nil {
		return nil, err
	}
	return &types.MsgSubmitForcedTxResponse{}, nil
}

// SubmitRecoveryProof verifies a real re-anchoring ZK proof
// (spec/README.md's §Re-anchoring via ZK-Proof of Recovery); if valid it
// advances the rolling checkpoint (LastAnchoredRoot) and latches
// RealProofSubmittedHeight. Never sets FSMState directly -- RECOVERING ->
// ANCHORED still fires only via CalculateNextState's hysteresis-gated
// pipeline (refreshReanchoringProofValid).
//
// Rolling: a proof covers up to N_MAX=256 headers
// (circuit/reanchoring/src/main.nr), so rt_new must match SOME tracked
// header's state_root (findHeaderByStateRoot), not the tip -- each accepted
// proof advances the checkpoint, letting a sequence cover an arbitrarily
// long interval.
func (k *MsgServerImpl) SubmitRecoveryProof(ctx context.Context, msg *types.MsgSubmitRecoveryProofRequest) (*types.MsgSubmitRecoveryProofResponse, error) {
	// 1. Verify proof math against the circuit's embedded verification key.
	if !k.VerifyZKProof(msg.ZkProof, msg.PublicInputs) {
		return nil, types.ErrInvalidZKProof
	}

	// 2. Bind rt_last/rt_new to tracked state so a previously valid proof
	// can't be replayed against unrelated chain history.
	if len(msg.PublicInputs) != 96 {
		return nil, types.ErrInvalidZKProof
	}
	rtLast, rtNew := msg.PublicInputs[0:32], msg.PublicInputs[32:64]

	trackedLast, err := k.LastAnchoredRoot.Get(ctx)
	if err != nil || !bytes.Equal(rtLast, trackedLast) {
		return nil, types.ErrInvalidZKProof
	}
	newCheckpointHeight, found, err := k.findHeaderByStateRoot(ctx, rtNew)
	if err != nil || !found {
		return nil, types.ErrInvalidZKProof
	}

	// 3. Advance the checkpoint and prune covered headers. Latch is set last
	// so a partial failure never points at an uncommitted checkpoint.
	if err := k.LastAnchoredRoot.Set(ctx, rtNew); err != nil {
		return nil, err
	}
	if err := k.pruneHeaderHistoryUpTo(ctx, newCheckpointHeight); err != nil {
		return nil, err
	}
	if err := k.RealProofSubmittedHeight.Set(ctx, newCheckpointHeight); err != nil {
		return nil, err
	}

	// Best-effort audit pointer (where the witness chain was published),
	// never verified -- a failure must not fail an already-checkpointed proof.
	if msg.DaCelestiaHeight > 0 {
		_ = k.RecoveryProofDAHeights.Set(ctx, newCheckpointHeight, msg.DaCelestiaHeight)
	}
	return &types.MsgSubmitRecoveryProofResponse{}, nil
}

// findHeaderByStateRoot returns the tracked header's height whose StateRoot
// equals root, if any -- matches rt_new by content, not exact tip.
func (k *Keeper) findHeaderByStateRoot(ctx context.Context, root []byte) (uint64, bool, error) {
	iter, err := k.HeaderHistory.Iterate(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer iter.Close()
	kvs, err := iter.KeyValues()
	if err != nil {
		return 0, false, err
	}
	for _, kv := range kvs {
		if bytes.Equal(kv.Value.StateRoot, root) {
			return kv.Key, true, nil
		}
	}
	return 0, false, nil
}

// pruneHeaderHistoryUpTo removes headers at or below height -- the just-verified
// segment. Unlike pruneHeaderHistory (whole interval on return to ANCHORED),
// this clears only a prefix, keeping later headers for the next proof.
func (k *Keeper) pruneHeaderHistoryUpTo(ctx context.Context, height uint64) error {
	iter, err := k.HeaderHistory.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer iter.Close()
	heights, err := iter.Keys()
	if err != nil {
		return err
	}
	for _, h := range heights {
		if h <= height {
			if err := k.HeaderHistory.Remove(ctx, h); err != nil {
				return err
			}
		}
	}
	return nil
}

// LatestTrackedHeader returns the tip (highest height) of the current
// SOVEREIGN/RECOVERING interval's HeaderHistory.
func (k *Keeper) LatestTrackedHeader(ctx context.Context) (uint64, types.RecoveryHeader, error) {
	iter, err := k.HeaderHistory.Iterate(ctx, nil)
	if err != nil {
		return 0, types.RecoveryHeader{}, err
	}
	defer iter.Close()
	kvs, err := iter.KeyValues()
	if err != nil {
		return 0, types.RecoveryHeader{}, err
	}
	if len(kvs) == 0 {
		return 0, types.RecoveryHeader{}, types.ErrInvalidZKProof
	}
	best := kvs[0]
	for _, kv := range kvs[1:] {
		if kv.Key > best.Key {
			best = kv
		}
	}
	return best.Key, best.Value, nil
}
