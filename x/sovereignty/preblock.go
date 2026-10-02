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
		ext, ok, err := DecodeAggregatedHealthCertificate(req.Txs[0])
		if err != nil || !ok {
			return &sdk.ResponsePreBlock{}, nil
		}
		if err := CommitFSMTransition(ctx, k, ext); err != nil {
			return nil, err
		}
		// req.Hash is this height's real block hash, only available here (not
		// in CommitFSMTransition, which tests also call directly with no ABCI
		// request). ctx.BlockHeader().LastBlockId is NOT a substitute -- it's
		// never populated here, unlike AppHash; an earlier version wrongly
		// assumed otherwise and silently wrote empty hashes.
		if types.WithdrawLocked(ext.FsmState) {
			if err := k.BlockHashByHeight.Set(ctx, uint64(ctx.BlockHeight()), req.Hash); err != nil {
				return nil, err
			}
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
// happened via DecodeAggregatedHealthCertificate).
func CommitFSMTransition(ctx sdk.Context, k *keeper.Keeper, ext *types.AggregatedHealthCertificate) error {
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
	if err := k.FSMState.Set(ctx, ext.FsmState); err != nil {
		return err
	}
	// ext.Healthy is the already-agreed value (ProcessProposal's check #1b),
	// never recomputed from this validator's live local sensors (CLAUDE.md).
	if err := k.SafeBlocks.Set(ctx, keeper.NextSafeBlocks(currState, ext.FsmState, safeBlocks, ext.Healthy, k.Params)); err != nil {
		return err
	}
	if err := k.SuspiciousDuration.Set(ctx, keeper.NextSuspiciousDuration(currState, ext.FsmState, suspiciousDuration, k.Params)); err != nil {
		return err
	}
	if err := k.UnhealthyStreak.Set(ctx, keeper.NextUnhealthyStreak(currState, ext.FsmState, unhealthyStreak, ext.Healthy)); err != nil {
		return err
	}
	if err := k.SuspiciousSafeBlocks.Set(ctx, keeper.NextSuspiciousSafeBlocks(currState, ext.FsmState, suspiciousSafeBlocks, ext.Healthy, k.Params)); err != nil {
		return err
	}
	// No local-sensor read -- unlike SafeBlocks/UnhealthyStreak above.
	if err := k.FailedRecoveryAttempts.Set(ctx, keeper.NextFailedRecoveryAttempts(currState, ext.FsmState, failedRecoveryAttempts, k.Params)); err != nil {
		return err
	}
	if err := k.HBtcAnchored.Set(ctx, ext.BtcCheckpointBlockHeight); err != nil {
		return err
	}
	// Snapshot the PRE-transition value (not the value about to be committed
	// below) the one block RECOVERING is entered -- keeper.go's
	// HEngramVerifiedAtRecoveryEntry doc explains why a fixed per-episode
	// reference is required.
	if currState != types.StateRecovering && ext.FsmState == types.StateRecovering {
		hEngramVerifiedBefore, _ := k.HEngramVerified.Get(ctx)
		if err := k.HEngramVerifiedAtRecoveryEntry.Set(ctx, hEngramVerifiedBefore); err != nil {
			return err
		}
		// Heights before this block form the SOVEREIGN interval whose data the
		// DA backfill must cover (keeper.DABackfillValid).
		if err := k.RecoveringEntryHeight.Set(ctx, uint64(ctx.BlockHeight())); err != nil {
			return err
		}
		// HeuristicProofLatched.doc: fresh episode, nothing latched yet.
		if err := k.HeuristicProofLatched.Set(ctx, false); err != nil {
			return err
		}
	}
	if err := k.HEngramVerified.Set(ctx, ext.DaPublishedBlockHeight); err != nil {
		return err
	}

	// h_btc_current: the quorum-certified median, re-derived deterministically
	// from ext.Certificates (same computation ProcessProposal already did),
	// written unconditionally every block -- leaving it stale round-skips
	// every proposal once a real checkpoint height exceeds it.
	metrics, _ := AggregateHealthCertificates(ctx, k, ctx.ChainID(), ext.Height, ext.Certificates)
	var hBtcCurrent uint64
	if metrics != nil {
		hBtcCurrent = metrics.BtcCurrentHeight
	}
	if err := k.HBtcCurrent.Set(ctx, hBtcCurrent); err != nil {
		return err
	}

	// Recomputed from THIS block's just-committed FSMState/HBtcAnchored, not
	// inside ExtendVote's throwaway CacheMultiStore (a dead-write bug class
	// that made this unreachable before). Must run before Step 4, which
	// overwrites the freshly-submitted-proof case back to false.
	if err := refreshReanchoringProofValid(ctx, k); err != nil {
		return err
	}

	// Step 4: mark a claimed ZK proof as submitted. Latched ONCE per episode
	// (HeuristicProofLatched's doc) -- SafeBlocks stays pinned at
	// HysteresisWait, so re-running this every block would re-pin
	// HBtcSubmitted to an ever-climbing value and hAnchored could never
	// catch up.
	if ext.FsmState == types.StateRecovering && len(ext.ZkProofRef) > 0 {
		latched, _ := k.HeuristicProofLatched.Get(ctx)
		if !latched {
			if err := k.HBtcSubmitted.Set(ctx, hBtcCurrent); err != nil {
				return err
			}
			if err := k.ReanchoringProofValid.Set(ctx, false); err != nil {
				return err
			}
			if err := k.HeuristicProofLatched.Set(ctx, true); err != nil {
				return err
			}
		}
	}

	// Step 5: on ANCHORED from RECOVERING, force-sync local sensors to kill
	// lingering local false alarms.
	if currState == types.StateRecovering && ext.FsmState == types.StateAnchored {
		if err := k.HEngramCurrent.Set(ctx, ext.DaPublishedBlockHeight); err != nil {
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
	// HeuristicProofLatched resets alongside it for the same reason (its own doc).
	if currState == types.StateRecovering && ext.FsmState != types.StateRecovering {
		if err := k.RealProofSubmittedHeight.Set(ctx, 0); err != nil {
			return err
		}
		if err := k.HeuristicProofLatched.Set(ctx, false); err != nil {
			return err
		}
	}

	// Step 6: track per-block header history for the ZK re-anchoring
	// circuit's witness, only while SOVEREIGN/RECOVERING. Accumulator chains
	// via HashHeader (main.nr's hash_header) so it can only advance via
	// real, agreed blocks.
	if types.WithdrawLocked(ext.FsmState) {
		if !types.WithdrawLocked(currState) {
			// First entry: rt_last is this block's incoming AppHash, the
			// pre-interval state headers[0].prev_hash must bind to;
			// LastAnchoredHeight lets SubmitRecoveryProof later enforce the
			// proof's claimed `count` against the real interval length.
			if err := k.LastAnchoredRoot.Set(ctx, types.ReduceToField(ctx.BlockHeader().AppHash)); err != nil {
				return err
			}
			if err := k.LastAnchoredHeight.Set(ctx, uint64(ctx.BlockHeight()-1)); err != nil {
				return err
			}
		}
		// Chains from the preceding tracked header, or LastAnchoredRoot when
		// HeaderHistory is empty (first header, or a prior proof pruned the
		// backlog up to the current tip mid-interval).
		prevAccumulator, err := k.LastAnchoredRoot.Get(ctx)
		if err != nil {
			return err
		}
		if _, latest, err := k.LatestTrackedHeader(ctx); err == nil {
			prevAccumulator = latest.Accumulator
		}

		stateRoot := types.ReduceToField(ctx.BlockHeader().AppHash)
		header := types.RecoveryHeader{
			FsmState:         ext.FsmState,
			WithdrawalLocked: true,
			StateRoot:        stateRoot,
			Accumulator:      types.HashHeader(prevAccumulator, ext.FsmState, true, stateRoot),
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

// refreshReanchoringProofValid ports UpdateSensors' reanchoring_proof_valid
// (spec/core/EngramFSM.tla:294-301). Runs after this block's FSMState/
// HBtcAnchored writes so its reads pick up THIS block's fresh values. Reads
// only committed keeper state, never a live sensor.
func refreshReanchoringProofValid(ctx sdk.Context, k *keeper.Keeper) error {
	currState, err := k.FSMState.Get(ctx)
	if err != nil {
		currState = types.StateAnchored
	}
	hAnchored, _ := k.HBtcAnchored.Get(ctx)
	hSubmitted, _ := k.HBtcSubmitted.Get(ctx)
	realProofSubmittedHeight, _ := k.RealProofSubmittedHeight.Get(ctx)

	realProofValid := false
	if realProofSubmittedHeight > 0 {
		if tipHeight, _, err := k.LatestTrackedHeader(ctx); err == nil && tipHeight-realProofSubmittedHeight <= k.Params.MaxUnprovenTailBlocks {
			realProofValid = true
		}
	}

	heuristicValid := hAnchored >= hSubmitted && hSubmitted > 0
	// ProofOnlyRecovery (types.Params doc) disables the heuristic fallback --
	// without this, a successful recovery can't be attributed to the ZK
	// pipeline specifically, since either path alone grants validity (E10).
	if k.Params.ProofOnlyRecovery {
		heuristicValid = false
	}
	valid := currState == types.StateRecovering && (heuristicValid || realProofValid)
	return k.ReanchoringProofValid.Set(ctx, valid)
}
