package keeper

import (
	"bytes"
	"context"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

type MsgServerImpl struct {
	*Keeper
}

func NewMsgServerImpl(k *Keeper) types.MsgServer {
	return &MsgServerImpl{Keeper: k}
}

// InjectFault: Dành cho test/thực nghiệm
func (k *MsgServerImpl) InjectFault(ctx context.Context, msg *types.MsgInjectFaultRequest) (*types.MsgInjectFaultResponse, error) {
	if msg.FaultInputs != nil {
		if err := k.Metrics.Set(ctx, msg.FaultInputs); err != nil {
			return nil, err
		}
	}
	return &types.MsgInjectFaultResponse{}, nil
}

// SubmitForcedTx ports SubmitToCelestiaDA (spec/core/EngramTendermint.tla:886-892):
// adds a tx to forced_tx_queue. Re-submitting an already-queued tx is a no-op
// (the spec's \E tx \in ValidValues \ forced_tx_queue guard means the action
// is simply not enabled for an already-present tx; collections.KeySet.Set is
// naturally idempotent, so this matches without an extra check).
func (k *MsgServerImpl) SubmitForcedTx(ctx context.Context, msg *types.MsgSubmitForcedTxRequest) (*types.MsgSubmitForcedTxResponse, error) {
	if err := k.ForcedTxQueue.Set(ctx, string(msg.Tx)); err != nil {
		return nil, err
	}
	return &types.MsgSubmitForcedTxResponse{}, nil
}

// SubmitRecoveryProof verifies a real re-anchoring ZK proof
// (spec/README.md's §Re-anchoring via ZK-Proof of Recovery) and, if valid,
// latches RealProofSubmittedHeight -- it does NOT set FSMState directly. See
// x/sovereignty/sensors_refresh.go's refreshReanchoringProofValid, the only
// consumer of that latch: RECOVERING -> ANCHORED still only ever fires
// through CalculateNextState's existing hysteresis-gated pipeline
// (SafeBlocks == HysteresisWait && ReanchoringProofValid) -- the same guard
// the BTC-anchor heuristic path already goes through. This closes a real
// safety bug: the previous version set FSMState=ANCHORED unconditionally,
// from ANY current state, on proof-math validity alone -- bypassing both
// StrictFSMTransitionSafety (e.g. a direct SOVEREIGN -> ANCHORED jump) and
// the hysteresis dwell time, via a second, unguarded FSM-state writer.
func (k *MsgServerImpl) SubmitRecoveryProof(ctx context.Context, msg *types.MsgSubmitRecoveryProofRequest) (*types.MsgSubmitRecoveryProofResponse, error) {
	// 1. Verify proof math against the circuit's embedded verification key.
	if !k.VerifyZKProof(msg.ZkProof, msg.PublicInputs) {
		return nil, types.ErrInvalidZKProof
	}

	// 2. Cross-check the proof's public inputs (rt_last, rt_new -- two
	// 32-byte big-endian Field values, matching circuit/reanchoring's
	// public_inputs file layout) against on-chain tracked state. Proof math
	// alone only proves "some self-consistent N-header chain links SOME
	// rt_last to SOME rt_new" -- without this check, any previously
	// generated valid proof (e.g. the one checked into
	// circuit/reanchoring/target/proof/) could be replayed against this
	// chain regardless of whether it reflects this chain's actual history.
	if len(msg.PublicInputs) != 64 {
		return nil, types.ErrInvalidZKProof
	}
	rtLast, rtNew := msg.PublicInputs[0:32], msg.PublicInputs[32:64]

	trackedLast, err := k.LastAnchoredRoot.Get(ctx)
	if err != nil || !bytes.Equal(rtLast, trackedLast) {
		return nil, types.ErrInvalidZKProof
	}
	tipHeight, tip, err := k.LatestTrackedHeader(ctx)
	if err != nil || !bytes.Equal(rtNew, tip.StateRoot) {
		return nil, types.ErrInvalidZKProof
	}

	// 3. Latch the verified proof, recording WHICH height it proved up to
	// (see RealProofSubmittedHeight's doc on keeper.go for why the height,
	// not just a bool, matters).
	if err := k.RealProofSubmittedHeight.Set(ctx, tipHeight); err != nil {
		return nil, err
	}
	return &types.MsgSubmitRecoveryProofResponse{}, nil
}

// LatestTrackedHeader returns the height and value of the highest-height
// entry in HeaderHistory -- the tip of the CURRENT SOVEREIGN/RECOVERING
// interval, i.e. the header whose state_root a real recovery proof's
// rt_new must match (SubmitRecoveryProof), and whose height
// RealProofSubmittedHeight must still equal for that proof to count as
// current (refreshReanchoringProofValid).
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
