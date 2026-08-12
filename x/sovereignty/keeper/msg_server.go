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
// queues tx into forced_tx_queue. Re-submitting an already-queued tx is a
// no-op (collections.KeySet.Set is naturally idempotent).
//
// Rejects content that doesn't decode as a real tx (via k.TxDecoder) --
// otherwise it could never satisfy IsCensoring's "included in req.Txs"
// check, deadlocking every future proposal once its ignored-round counter
// reaches MaxIgnoreRounds.
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
// (spec/README.md's §Re-anchoring via ZK-Proof of Recovery) and, if valid,
// advances the rolling checkpoint (LastAnchoredRoot) and latches
// RealProofSubmittedHeight. It never sets FSMState directly -- RECOVERING ->
// ANCHORED still only fires through CalculateNextState's own
// hysteresis-gated pipeline (see refreshReanchoringProofValid).
//
// Rolling, not fixed-interval: the circuit covers up to N_MAX=256 headers
// per proof (circuit/reanchoring/src/main.nr), but a real interval can
// outgrow that. So rt_new only needs to match SOME tracked header's
// state_root (via findHeaderByStateRoot), not the absolute tip -- the proof
// itself already guarantees `count` real headers connect rt_last to that
// point. Each accepted proof advances the checkpoint and prunes everything
// at or below it, letting a sequence of proofs cover an arbitrarily long
// interval.
func (k *MsgServerImpl) SubmitRecoveryProof(ctx context.Context, msg *types.MsgSubmitRecoveryProofRequest) (*types.MsgSubmitRecoveryProofResponse, error) {
	// 1. Verify proof math against the circuit's embedded verification key.
	if !k.VerifyZKProof(msg.ZkProof, msg.PublicInputs) {
		return nil, types.ErrInvalidZKProof
	}

	// 2. Cross-check public inputs (rt_last, rt_new, count -- three 32-byte
	// Field values, spec/README.md's x = (rt_last, rt_new, n)) against
	// on-chain tracked state, so a previously valid proof can't be replayed
	// against unrelated chain history.
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

	// 3. Advance the checkpoint and prune what the next proof won't need.
	// RealProofSubmittedHeight is set last so a partial failure never
	// leaves the latch pointing at an uncommitted checkpoint.
	if err := k.LastAnchoredRoot.Set(ctx, rtNew); err != nil {
		return nil, err
	}
	if err := k.pruneHeaderHistoryUpTo(ctx, newCheckpointHeight); err != nil {
		return nil, err
	}
	if err := k.RealProofSubmittedHeight.Set(ctx, newCheckpointHeight); err != nil {
		return nil, err
	}

	// Record where the witness header chain was published, if the caller
	// says it was -- a pure audit pointer, never verified here. Best-effort:
	// a failure to record it must not fail an otherwise-valid, already-
	// checkpointed proof.
	if msg.DaCelestiaHeight > 0 {
		_ = k.RecoveryProofDAHeights.Set(ctx, newCheckpointHeight, msg.DaCelestiaHeight)
	}
	return &types.MsgSubmitRecoveryProofResponse{}, nil
}

// findHeaderByStateRoot returns the height of the tracked header whose
// StateRoot equals root, and whether one was found -- lets
// SubmitRecoveryProof match a proof's rt_new by content instead of
// requiring an exact tip match.
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

// pruneHeaderHistoryUpTo removes tracked headers at or below height -- the
// segment a just-verified proof covered. Unlike pruneHeaderHistory (which
// clears the entire interval on a full return to ANCHORED), this only
// clears a prefix, leaving later headers as the start of the next segment.
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

// LatestTrackedHeader returns the height and value of the highest entry in
// HeaderHistory -- the tip of the current SOVEREIGN/RECOVERING interval.
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
