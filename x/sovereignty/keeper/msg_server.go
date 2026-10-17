package keeper

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

type MsgServerImpl struct {
	*Keeper

	verifyMu     sync.Mutex
	verifyHeight int64
	verifyCount  int
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

// SubmitRecoveryProof verifies a real ZK re-anchoring proof (spec/README.md's Re-anchoring
// via ZK-Proof of Recovery); if valid it advances the rolling checkpoint and latches
// RealProofSubmittedHeight -- never sets FSMState directly, only refreshReanchoringProofValid does.
func (k *MsgServerImpl) SubmitRecoveryProof(ctx context.Context, msg *types.MsgSubmitRecoveryProofRequest) (*types.MsgSubmitRecoveryProofResponse, error) {
	// 1. Bind rt_last/rt_new/count/boundary public inputs to tracked state so a proof can't be
	// replayed against unrelated history or claim fewer headers than it covers. Public-input
	// layout (circuit/reanchoring/src/main.nr): rt_last, rt_new, count, first_height,
	// first/last_block_hash, validator_set_hash -- 7x32B fields, matching bb's layout.
	if len(msg.PublicInputs) != 224 {
		return nil, rejectProof(ctx, "public_inputs_length")
	}
	rtLast, rtNew := msg.PublicInputs[0:32], msg.PublicInputs[32:64]
	count := binary.BigEndian.Uint32(msg.PublicInputs[92:96])
	firstHeight := binary.BigEndian.Uint64(msg.PublicInputs[120:128])
	firstBlockHash := msg.PublicInputs[128:160]
	lastBlockHash := msg.PublicInputs[160:192]
	validatorSetHash := msg.PublicInputs[192:224]

	trackedLast, err := k.LastAnchoredRoot.Get(ctx)
	if err != nil || !bytes.Equal(rtLast, trackedLast) {
		return nil, rejectProof(ctx, "rt_last")
	}
	lastAnchoredHeight, err := k.LastAnchoredHeight.Get(ctx)
	if err != nil {
		return nil, rejectProof(ctx, "last_anchored_height_missing")
	}
	newCheckpointHeight, found, err := k.findHeaderByAccumulator(ctx, rtNew)
	if err != nil || !found {
		return nil, rejectProof(ctx, "rt_new_not_found")
	}
	if uint64(count) != newCheckpointHeight-lastAnchoredHeight {
		return nil, rejectProof(ctx, "count")
	}

	// 2b. Boundary-block binding (circuit/README.md's Scope section): the proof's claimed
	// first/last block and validator set must match what this chain actually recorded, tying
	// the proof to real >=2/3-signed CometBFT blocks, not a self-consistent fabricated chain.
	//
	// first_height is lastAnchoredHeight+1 (the interval's first real WithdrawLocked block) --
	// lastAnchoredHeight itself is the pre-interval ANCHORED block, which has no BlockHashByHeight entry.
	if uint64(firstHeight) != lastAnchoredHeight+1 {
		return nil, rejectProof(ctx, "first_height")
	}
	trackedFirstBlockHash, err := k.BlockHashByHeight.Get(ctx, uint64(firstHeight))
	if err != nil || !bytes.Equal(firstBlockHash, types.ReduceToField(trackedFirstBlockHash)) {
		return nil, rejectProof(ctx, "first_block_hash")
	}
	trackedLastBlockHash, err := k.BlockHashByHeight.Get(ctx, newCheckpointHeight)
	if err != nil || !bytes.Equal(lastBlockHash, types.ReduceToField(trackedLastBlockHash)) {
		return nil, rejectProof(ctx, "last_block_hash")
	}
	trackedValidatorSetHash, err := k.ValidatorSetHash.Get(ctx)
	if err != nil || !bytes.Equal(validatorSetHash, types.ReduceToField(trackedValidatorSetHash)) {
		return nil, rejectProof(ctx, "validator_set_hash")
	}

	// 2. Verify proof math against the circuit's embedded verification key. Last on purpose:
	// the bindings above are store lookups, the verification is ~20 ms of UltraHonk, so a proof
	// whose inputs do not match tracked state is rejected without paying for it. With no fee or
	// signature on this tx, at most maxProofVerifiesPerBlock proofs are verified per block.
	if !k.allowProofVerify(ctx) {
		return nil, rejectProof(ctx, "verify_cap_per_block")
	}
	if !k.VerifyZKProof(msg.ZkProof, msg.PublicInputs) {
		return nil, rejectProof(ctx, "ultrahonk_verify")
	}

	// 3. Advance the checkpoint and prune covered headers. Latch is set last
	// so a partial failure never points at an uncommitted checkpoint.
	if err := k.LastAnchoredRoot.Set(ctx, rtNew); err != nil {
		return nil, err
	}
	if err := k.LastAnchoredHeight.Set(ctx, newCheckpointHeight); err != nil {
		return nil, err
	}
	if err := k.pruneHeaderHistoryUpTo(ctx, newCheckpointHeight); err != nil {
		return nil, err
	}
	if err := k.pruneBlockHashesUpTo(ctx, newCheckpointHeight); err != nil {
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
	logProof(ctx, "recovery proof accepted", "first_height", firstHeight, "last_height", newCheckpointHeight, "count", count)
	return &types.MsgSubmitRecoveryProofResponse{}, nil
}

// rejectProof returns ErrInvalidZKProof and logs which check refused the proof (the error itself is the same for every
// check). Logging only: no state or consensus effect.
func rejectProof(ctx context.Context, check string) error {
	logProof(ctx, "recovery proof rejected", "check", check)
	return types.ErrInvalidZKProof
}

// logProof writes one line to the node log; logging only, no state or consensus effect.
func logProof(ctx context.Context, msg string, kv ...any) {
	sctx, ok := ctx.(sdk.Context)
	if !ok {
		sctx, ok = ctx.Value(sdk.SdkContextKey).(sdk.Context)
	}
	if ok && sctx.Logger() != nil {
		sctx.Logger().Info(msg, kv...)
	}
}

// findHeaderByAccumulator matches rt_new by content (not the tip) against a tracked header's
// Accumulator -- a proof covers up to N_MAX=256 headers, so rt_new may land mid-history, and
// Accumulator is what only a real hash_header chain from LastAnchoredRoot could have produced.
func (k *Keeper) findHeaderByAccumulator(ctx context.Context, root []byte) (uint64, bool, error) {
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
		if bytes.Equal(kv.Value.Accumulator, root) {
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

// pruneBlockHashesUpTo removes BlockHashByHeight entries at or below height
// -- mirrors pruneHeaderHistoryUpTo; a height at or below the new checkpoint
// can never be referenced by a future boundary-binding check again (both
// firstHeight and newCheckpointHeight only ever move forward).
func (k *Keeper) pruneBlockHashesUpTo(ctx context.Context, height uint64) error {
	iter, err := k.BlockHashByHeight.Iterate(ctx, nil)
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
			if err := k.BlockHashByHeight.Remove(ctx, h); err != nil {
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

// maxProofVerifiesPerBlock bounds the UltraHonk verifications one block can spend on
// SubmitRecoveryProof; a legitimate recovery needs one (a few with aggregation).
const maxProofVerifiesPerBlock = 8

// allowProofVerify counts verifications per block height in memory. The count follows the block's
// transaction order, so every validator reaches the same decision when it executes the block.
func (k *MsgServerImpl) allowProofVerify(ctx context.Context) bool {
	sctx, ok := ctx.Value(sdk.SdkContextKey).(sdk.Context)
	if !ok {
		return true
	}
	h := sctx.BlockHeight()
	k.verifyMu.Lock()
	defer k.verifyMu.Unlock()
	if k.verifyHeight != h {
		k.verifyHeight, k.verifyCount = h, 0
	}
	k.verifyCount++
	return k.verifyCount <= maxProofVerifiesPerBlock
}
