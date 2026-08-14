package sovereignty

import (
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewPreBlocker mirrors ServerUponProposalInPrecommitNoDecision
// (spec/core/EngramServer.tla:135-189): commits the ALREADY-AGREED fsm_state
// from the decided block's Txs[0], never recomputed locally -- "sensors
// propose, consensus decides."
//
// Never call RefreshMetrics (live, node-local sensor data) here: it puts
// each validator's local view into AppHash, so honest validators with
// slightly different local readings diverge (reproduced live on 4 nodes).
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

// updateForcedTxTracking ports UpdateIgnoredRounds
// (spec/core/EngramTendermint.tla:493-503), once per finalized block (vanilla
// ABCI 2.0 exposes no round number). No-op when forced_tx_queue is empty.
//
// Dequeues a forced tx once included instead of resetting its counter: txs are
// one-shot, so a leftover queued tx would trip IsCensoring (check #0) forever.
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
	// ext.Healthy is the already-agreed value (ProcessProposal's check #1b),
	// never recomputed from this validator's live local sensors (CLAUDE.md).
	if err := k.SafeBlocks.Set(ctx, keeper.NextSafeBlocks(currState, ext.FSMState, safeBlocks, ext.Healthy, k.Params)); err != nil {
		return err
	}
	if err := k.SuspiciousDuration.Set(ctx, keeper.NextSuspiciousDuration(currState, ext.FSMState, suspiciousDuration, k.Params)); err != nil {
		return err
	}
	if err := k.UnhealthyStreak.Set(ctx, keeper.NextUnhealthyStreak(currState, ext.FSMState, unhealthyStreak, ext.Healthy)); err != nil {
		return err
	}
	if err := k.SuspiciousSafeBlocks.Set(ctx, keeper.NextSuspiciousSafeBlocks(currState, ext.FSMState, suspiciousSafeBlocks, ext.Healthy, k.Params)); err != nil {
		return err
	}
	// No local-sensor read -- unlike SafeBlocks/UnhealthyStreak above.
	if err := k.FailedRecoveryAttempts.Set(ctx, keeper.NextFailedRecoveryAttempts(currState, ext.FSMState, failedRecoveryAttempts, k.Params)); err != nil {
		return err
	}
	if err := k.HBtcAnchored.Set(ctx, ext.BTCReceipt.CheckpointBlockHeight); err != nil {
		return err
	}
	if err := k.HEngramVerified.Set(ctx, ext.DAReceipt.PublishedBlockHeight); err != nil {
		return err
	}

	// Step 4: mark a claimed ZK proof as submitted (pending BTC confirmation).
	if ext.FSMState == types.StateRecovering && len(ext.ZKProofRef) > 0 {
		hBtcCurrent, _ := k.HBtcCurrent.Get(ctx)
		if err := k.HBtcSubmitted.Set(ctx, hBtcCurrent); err != nil {
			return err
		}
		if err := k.ReanchoringProofValid.Set(ctx, false); err != nil {
			return err
		}
	}

	// Step 5: on ANCHORED from RECOVERING, force-sync local sensors to kill
	// lingering local false alarms.
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

	// A proof only applies to the interval it was proven against -- reset on
	// leaving RECOVERING (see refreshReanchoringProofValid, the only reader).
	if currState == types.StateRecovering && ext.FSMState != types.StateRecovering {
		if err := k.RealProofSubmittedHeight.Set(ctx, 0); err != nil {
			return err
		}
	}

	// Step 6: track per-block header history for the ZK re-anchoring circuit's
	// witness (spec/README.md's §Re-anchoring via ZK-Proof of Recovery), only
	// while SOVEREIGN/RECOVERING. StateRoot is CometBFT's real AppHash.
	if types.WithdrawLocked(ext.FSMState) {
		if !types.WithdrawLocked(currState) {
			// First entry into the interval: rt_last is this block's incoming
			// AppHash, the pre-interval state headers[0].prev_hash must bind to.
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
		// Interval ended (back to ANCHORED): prune; next interval starts fresh.
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

// recordDetectedEvidence logs req.Misbehavior -- CometBFT's own
// DuplicateVote/LightClientAttack detections (docs/EXPERIMENT.md's E8
// "Double-signing" row). Safe to commit: deterministic across validators.
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
