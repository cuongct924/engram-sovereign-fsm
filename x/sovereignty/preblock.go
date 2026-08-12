package sovereignty

import (
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewPreBlocker builds the sdk.PreBlocker for this module, mirroring
// ServerUponProposalInPrecommitNoDecision (spec/core/EngramServer.tla:135-189):
// commits the ALREADY-AGREED fsm_state from the decided block's extended
// proposal (Txs[0]), never recomputed locally -- "sensors propose,
// consensus decides."
//
// Must never call RefreshMetrics (live, node-LOCAL sensor data) from here:
// committing it would make each validator's local view part of AppHash,
// causing honest validators with even slightly different local readings
// (e.g. differing P2P peer sets) to diverge on AppHash for the identical
// agreed block -- a real consensus-safety failure reproduced on a live
// 4-node testnet. This function commits only fields already
// deterministically embedded in the agreed proposal (ext.BTCReceipt/
// ext.DAReceipt, via CommitFSMTransition below).
func NewPreBlocker(k *keeper.Keeper) sdk.PreBlocker {
	return func(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
		if err := recordDetectedEvidence(ctx, k, req); err != nil {
			return nil, err
		}
		if err := updateForcedTxTracking(ctx, k, req.Txs); err != nil {
			return nil, err
		}
		if len(req.Txs) == 0 {
			return &sdk.ResponsePreBlock{}, nil
		}
		ext, ok, err := DecodeExtendedProposal(req.Txs[0])
		if err != nil || !ok {
			return &sdk.ResponsePreBlock{}, nil
		}
		if err := CommitFSMTransition(ctx, k, ext); err != nil {
			return nil, err
		}
		return &sdk.ResponsePreBlock{}, nil
	}
}

// updateForcedTxTracking ports UpdateIgnoredRounds (spec/core/EngramTendermint.tla:493-503),
// run once per finalized block (vanilla ABCI 2.0 exposes no round number to
// app hooks). No-op when forced_tx_queue is empty.
//
// Dequeues a forced tx entirely once included, rather than just resetting
// its ignored-round counter: this app's txs are one-shot (no resubmission),
// so a tx left queued after its only possible inclusion has
// included[tx]==false forever, permanently tripping IsCensoring (check #0)
// on every future proposal from every validator.
func updateForcedTxTracking(ctx sdk.Context, k *keeper.Keeper, txs [][]byte) error {
	forcedTxQueue, err := k.ForcedTxQueueSlice(ctx)
	if err != nil || len(forcedTxQueue) == 0 {
		return err
	}
	included := make(map[string]bool, len(txs))
	for _, tx := range txs {
		included[string(tx)] = true
	}
	next := types.NextIgnoredRounds(forcedTxQueue, k.IgnoredRoundsMap(ctx, forcedTxQueue), included, k.Params.MaxIgnoreRounds)
	for tx, count := range next {
		if included[tx] {
			if err := k.ForcedTxQueue.Remove(ctx, tx); err != nil {
				return err
			}
			if err := k.TxIgnoredRounds.Remove(ctx, tx); err != nil {
				return err
			}
			continue
		}
		if err := k.TxIgnoredRounds.Set(ctx, tx, count); err != nil {
			return err
		}
	}
	return nil
}

// CommitFSMTransition ports ServerUponProposalInPrecommitNoDecision's state
// writes (spec/core/EngramServer.tla:135-189, steps 3-5; steps 1-2 already
// happened via DecodeExtendedProposal).
func CommitFSMTransition(ctx sdk.Context, k *keeper.Keeper, ext ExtendedProposal) error {
	currState, err := k.FSMState.Get(ctx)
	if err != nil {
		currState = types.StateAnchored
	}
	safeBlocks, _ := k.SafeBlocks.Get(ctx)
	suspiciousDuration, _ := k.SuspiciousDuration.Get(ctx)
	suspiciousSafeBlocks, _ := k.SuspiciousSafeBlocks.Get(ctx)
	unhealthyStreak, _ := k.UnhealthyStreak.Get(ctx)
	failedRecoveryAttempts, _ := k.FailedRecoveryAttempts.Get(ctx)

	// Step 3: drive the FSM transition to the agreed state and update anchored heights.
	if err := k.FSMState.Set(ctx, ext.FSMState); err != nil {
		return err
	}
	// ext.Healthy is the already-agreed value (validated against every
	// honest validator's own sensors in ProcessProposal's check #1b) -- NOT
	// recomputed from this validator's own live local sensors, per
	// CLAUDE.md's rule against writing live local sensor reads into
	// committed state.
	if err := k.SafeBlocks.Set(ctx, keeper.NextSafeBlocks(currState, ext.FSMState, safeBlocks, ext.Healthy, k.Params)); err != nil {
		return err
	}
	if err := k.SuspiciousDuration.Set(ctx, keeper.NextSuspiciousDuration(currState, ext.FSMState, suspiciousDuration, k.Params)); err != nil {
		return err
	}
	if err := k.UnhealthyStreak.Set(ctx, keeper.NextUnhealthyStreak(currState, ext.FSMState, unhealthyStreak, ext.Healthy)); err != nil {
		return err
	}
	// ext.Healthy is the already-agreed value, same rationale as SafeBlocks above.
	if err := k.SuspiciousSafeBlocks.Set(ctx, keeper.NextSuspiciousSafeBlocks(currState, ext.FSMState, suspiciousSafeBlocks, ext.Healthy, k.Params)); err != nil {
		return err
	}
	// NextFailedRecoveryAttempts depends only on the already-agreed
	// (currState, ext.FSMState) transition and k.Params -- no local-sensor
	// read needed here, unlike SafeBlocks/UnhealthyStreak above.
	if err := k.FailedRecoveryAttempts.Set(ctx, keeper.NextFailedRecoveryAttempts(currState, ext.FSMState, failedRecoveryAttempts, k.Params)); err != nil {
		return err
	}
	if err := k.HBtcAnchored.Set(ctx, ext.BTCReceipt.CheckpointBlockHeight); err != nil {
		return err
	}
	if err := k.HEngramVerified.Set(ctx, ext.DAReceipt.PublishedBlockHeight); err != nil {
		return err
	}

	// Step 4: ZK proof submission tracking -- mark as submitted (pending
	// Bitcoin confirmation) once a proof was claimed while entering/staying RECOVERING.
	if ext.FSMState == types.StateRecovering && len(ext.ZKProofRef) > 0 {
		hBtcCurrent, _ := k.HBtcCurrent.Get(ctx)
		if err := k.HBtcSubmitted.Set(ctx, hBtcCurrent); err != nil {
			return err
		}
		if err := k.ReanchoringProofValid.Set(ctx, false); err != nil {
			return err
		}
	}

	// Step 5: force-sync local sensors when the network majority reaches
	// ANCHORED from RECOVERING, suppressing any lingering local false alarms.
	if currState == types.StateRecovering && ext.FSMState == types.StateAnchored {
		if err := k.HBtcCurrent.Set(ctx, ext.BTCReceipt.CheckpointBlockHeight); err != nil {
			return err
		}
		if err := k.HEngramCurrent.Set(ctx, ext.DAReceipt.PublishedBlockHeight); err != nil {
			return err
		}
		metrics, err := k.Metrics.Get(ctx)
		if err != nil || metrics == nil {
			metrics = &types.PeripheralMetrics{}
		}
		metrics.IsDasFailed = false
		metrics.IsAttestationFailed = false
		if err := k.Metrics.Set(ctx, metrics); err != nil {
			return err
		}
	}

	// Step 5b: a real ZK proof only applies to the SOVEREIGN/RECOVERING
	// interval it was proven against -- reset once RECOVERING is left, so a
	// stale latch can't apply to a new interval (see
	// refreshReanchoringProofValid, the only reader).
	if currState == types.StateRecovering && ext.FSMState != types.StateRecovering {
		if err := k.RealProofSubmittedHeight.Set(ctx, 0); err != nil {
			return err
		}
	}

	// Step 6: track per-block header history for the ZK re-anchoring
	// circuit's witness (spec/README.md's §Re-anchoring via ZK-Proof of
	// Recovery), only while SOVEREIGN/RECOVERING. StateRoot is CometBFT's
	// real AppHash, not the keeper's SMT (see types.RecoveryHeader's doc).
	if types.WithdrawLocked(ext.FSMState) {
		if !types.WithdrawLocked(currState) {
			// Entering the interval for the first time: rt_last is this
			// block's incoming AppHash, the state right before the interval
			// starts, which headers[0].prev_hash must bind to.
			if err := k.LastAnchoredRoot.Set(ctx, types.ReduceToField(ctx.BlockHeader().AppHash)); err != nil {
				return err
			}
		}
		header := types.RecoveryHeader{
			FsmState:         ext.FSMState,
			WithdrawalLocked: true,
			StateRoot:        types.ReduceToField(ctx.BlockHeader().AppHash),
		}
		if err := k.HeaderHistory.Set(ctx, uint64(ctx.BlockHeight()), header); err != nil {
			return err
		}
	} else if types.WithdrawLocked(currState) {
		// Interval just ended (back to ANCHORED): a future interval starts
		// its own LastAnchoredRoot from scratch above.
		if err := pruneHeaderHistory(ctx, k); err != nil {
			return err
		}
	}

	return nil
}

// pruneHeaderHistory clears HeaderHistory once a SOVEREIGN/RECOVERING
// interval ends -- only the CURRENT interval's headers are ever needed to
// build a re-anchoring proof against.
func pruneHeaderHistory(ctx sdk.Context, k *keeper.Keeper) error {
	iter, err := k.HeaderHistory.Iterate(ctx, nil)
	if err != nil {
		return err
	}
	defer iter.Close()
	heights, err := iter.Keys()
	if err != nil {
		return err
	}
	for _, height := range heights {
		if err := k.HeaderHistory.Remove(ctx, height); err != nil {
			return err
		}
	}
	return nil
}

// recordDetectedEvidence logs req.Misbehavior -- CometBFT's own evidence
// pool's DuplicateVote/LightClientAttack detections (docs/EXPERIMENT.md's
// E8 "Double-signing" row). Safe to commit: Misbehavior is part of
// RequestFinalizeBlock itself, deterministic across every honest validator,
// exactly like req.Txs (unlike a live local sensor read; see NewPreBlocker's doc).
func recordDetectedEvidence(ctx sdk.Context, k *keeper.Keeper, req *abci.RequestFinalizeBlock) error {
	for _, m := range req.Misbehavior {
		record := types.EvidenceRecord{
			Type:             types.MisbehaviorTypeName(int32(m.Type)),
			ValidatorAddress: m.Validator.Address,
			ValidatorPower:   m.Validator.Power,
			OffenseHeight:    m.Height,
			OffenseTime:      m.Time,
			DetectedAtHeight: req.Height,
		}
		if err := k.LastDetectedEvidence.Set(ctx, record); err != nil {
			return err
		}
		count, err := k.DetectedEvidenceCount.Get(ctx)
		if err != nil {
			count = 0
		}
		if err := k.DetectedEvidenceCount.Set(ctx, count+1); err != nil {
			return err
		}
		fmt.Printf("engramd: SLASHABLE EVIDENCE DETECTED type=%s validator=%X offense_height=%d detected_at_height=%d (latency=%d blocks)\n",
			record.Type, record.ValidatorAddress, record.OffenseHeight, record.DetectedAtHeight, record.DetectedAtHeight-record.OffenseHeight)
	}
	return nil
}
