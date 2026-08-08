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
// advances the rolling checkpoint (LastAnchoredRoot) and latches
// RealProofSubmittedHeight -- it does NOT set FSMState directly. See
// x/sovereignty/sensors_refresh.go's refreshReanchoringProofValid, the only
// consumer of that latch: RECOVERING -> ANCHORED still only ever fires
// through CalculateNextState's existing hysteresis-gated pipeline
// (SafeBlocks == HysteresisWait && ReanchoringProofValid) -- the same guard
// the BTC-anchor heuristic path already goes through. This closes a real
// safety bug: the previous version set FSMState=ANCHORED unconditionally,
// from ANY current state, on proof-math validity alone -- bypassing both
// StrictFSMTransitionSafety (e.g. a direct SOVEREIGN -> ANCHORED jump) and
// the hysteresis dwell time, via a second, unguarded FSM-state writer.
//
// Rolling checkpoint (not a fixed interval-start-to-tip proof): the
// circuit's N is fixed at compile time (circuit/reanchoring/src/main.nr's
// `global N: u32 = 4`), but a real SOVEREIGN/RECOVERING interval's length is
// environment-controlled and unbounded -- requiring one proof to span the
// ENTIRE interval (rt_last = the point ANCHORED was last left, rt_new = the
// CURRENT tip) makes the circuit permanently unusable the moment the
// interval exceeds N blocks, since the tip keeps moving before a proof can
// even be built. Confirmed live: HeaderHistory grew past 2,000 tracked
// blocks across a single never-yet-ANCHORED run this session, with the
// N=4 circuit unable to prove any of it.
//
// Fixed by NOT requiring rt_new to match the absolute tip: instead, rt_new
// only needs to match the state_root of SOME tracked header (found via
// findHeaderByStateRoot) -- the proof itself, having already verified
// against the fixed-N circuit's VK above, is what guarantees exactly N real
// headers connect rt_last to that point (Go never needs to know or hardcode
// N to trust this). Once a match is found, the checkpoint (LastAnchoredRoot)
// advances to it and everything at or below that height is pruned (a FUTURE
// proof only ever needs to link forward from the new checkpoint). This lets
// checkpoints advance every N blocks throughout an arbitrarily long
// interval via a sequence of real, independently-verified N-sized proofs,
// rather than needing one proof to catch the tip in a single-block-wide
// window. refreshReanchoringProofValid's existing "proof height == current
// tip height" check is UNCHANGED and still correct under this scheme: it
// naturally becomes true (and can trigger a real ANCHORED transition,
// gated on SafeBlocks == HysteresisWait as before) exactly when a
// checkpoint-advancing proof happens to land with no further headers having
// been appended since -- i.e. the interval has both recovered AND been
// fully proven, block for block, from the last real anchor to the tip.
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
	newCheckpointHeight, found, err := k.findHeaderByStateRoot(ctx, rtNew)
	if err != nil || !found {
		return nil, types.ErrInvalidZKProof
	}

	// 3. Advance the rolling checkpoint and prune everything the next proof
	// will no longer need. Order matters: RealProofSubmittedHeight is set
	// last so a failure partway through never leaves the latch pointing at
	// a checkpoint that wasn't actually committed.
	if err := k.LastAnchoredRoot.Set(ctx, rtNew); err != nil {
		return nil, err
	}
	if err := k.pruneHeaderHistoryUpTo(ctx, newCheckpointHeight); err != nil {
		return nil, err
	}
	if err := k.RealProofSubmittedHeight.Set(ctx, newCheckpointHeight); err != nil {
		return nil, err
	}
	return &types.MsgSubmitRecoveryProofResponse{}, nil
}

// findHeaderByStateRoot returns the height of the tracked header whose
// StateRoot equals root, and whether one was found. Used by
// SubmitRecoveryProof to locate a proof's rt_new within this validator's
// own tracked history (see that function's doc for why matching by content
// rather than requiring an exact tip match is what makes rolling,
// mid-interval checkpoint advances possible).
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

// pruneHeaderHistoryUpTo removes every tracked header at or below height --
// the segment a just-verified proof covered, which no future proof will
// ever need to reference again (a future proof only links forward from the
// new checkpoint). Unlike x/sovereignty's pruneHeaderHistory (which clears
// the ENTIRE tracked interval on a full return to ANCHORED), this only
// clears a prefix, leaving any already-accumulated later headers in place
// as the start of the next segment.
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
