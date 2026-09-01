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
		// BlockHashByHeight: this height's own real CometBFT block hash --
		// req.Hash ("the merkle root hash of the fields of the decided
		// block", abci/types/types.pb.go's RequestFinalizeBlock doc). Lives
		// here (not CommitFSMTransition) because req.Hash is only available
		// via the real ABCI request; CommitFSMTransition is also called
		// directly by tests/the in-process e2e BeginBlocker path with no
		// such request. ctx.BlockHeader().LastBlockId is NOT a substitute:
		// that field is never populated in this context (RequestFinalizeBlock
		// carries no LastBlockId at all, unlike AppHash, which BaseApp
		// threads through from its own prior response) -- an earlier
		// version of this code wrongly assumed it behaved the same way and
		// silently wrote empty hashes. What SubmitRecoveryProof later reads
		// for the first/last real header's boundary-signature binding
		// (keeper.go's BlockHashByHeight doc); kept general (any tracked
		// height, not just two fixed endpoints) so a checkpoint's own
		// height doesn't need to already be known before it can be proven.
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
	// HEngramVerifiedAtRecoveryEntry: snapshot the PRE-transition value (not
	// ext.DaPublishedBlockHeight, the value about to be committed below) the
	// one block RECOVERING is first entered -- see its keeper.go doc for why
	// a fixed per-episode reference, not the ratcheting committed value, is
	// required for the zk_proof_ref freshness gate to be satisfiable at all.
	if currState != types.StateRecovering && ext.FsmState == types.StateRecovering {
		hEngramVerifiedBefore, _ := k.HEngramVerified.Get(ctx)
		if err := k.HEngramVerifiedAtRecoveryEntry.Set(ctx, hEngramVerifiedBefore); err != nil {
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

	// h_btc_current: the quorum-certified median (metrics.BtcCurrentHeight,
	// re-derived deterministically from ext.Certificates -- the SAME
	// computation PrepareProposal/ProcessProposal already did this round),
	// written unconditionally every block. See PeripheralMetrics.
	// btc_current_height's doc: leaving this committed value stale (only
	// updated on the rare RECOVERING->ANCHORED edge, as the pre-vote-
	// extension design did) round-skips every proposal forever once a real,
	// growing checkpoint height first exceeds the frozen h_btc_current.
	metrics, _ := AggregateHealthCertificates(ctx, k, ctx.ChainID(), ext.Height, ext.Certificates)
	var hBtcCurrent uint64
	if metrics != nil {
		hBtcCurrent = metrics.BtcCurrentHeight
	}
	if err := k.HBtcCurrent.Set(ctx, hBtcCurrent); err != nil {
		return err
	}

	// reanchoring_proof_valid: recomputed from THIS block's just-committed
	// FSMState/HBtcAnchored above (see refreshReanchoringProofValid's doc --
	// moved here from RefreshMetrics, which only ever ran inside ExtendVote's
	// throwaway CacheMultiStore and so could set this to true, but never make
	// it stick: the RECOVERING->ANCHORED transition (gated on this value) was
	// consequently unreachable in real committed state -- reproduced live,
	// the third instance of the h_btc_current dead-write bug class in this
	// file). Must run before Step 4 below, which unconditionally overwrites
	// this block's own freshly-submitted-proof case back to false.
	if err := refreshReanchoringProofValid(ctx, k); err != nil {
		return err
	}

	// Step 4: mark a claimed ZK proof as submitted (pending BTC confirmation).
	// Latched ONCE per episode (HeuristicProofLatched's doc): SafeBlocks caps
	// at HysteresisWait and stays there, so a non-empty ZkProofRef keeps
	// showing up on every subsequent block too (ProcessProposal's check #5
	// requires it as long as SafeBlocks==HysteresisWait) -- re-running this
	// body every time would re-pin HBtcSubmitted to the current, ever-
	// climbing h_btc_current and reset ReanchoringProofValid to false every
	// block, so hAnchored (BTC-confirmation-lagged) could never catch up.
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

	// Step 6: track per-block header history for the ZK re-anchoring circuit's
	// witness (spec/README.md's §Re-anchoring via ZK-Proof of Recovery), only
	// while SOVEREIGN/RECOVERING. StateRoot is CometBFT's real AppHash;
	// Accumulator is HashHeader chained from LastAnchoredRoot -- the value a
	// real, honest witness built from this exact history would produce
	// (circuit/reanchoring/src/main.nr's hash_header). Only consensus ever
	// writes here, so the accumulator can't be advanced except by real,
	// agreed blocks -- see circuit/reanchoring/src/main.nr's rt_new-binding
	// doc for why this matters.
	if types.WithdrawLocked(ext.FsmState) {
		if !types.WithdrawLocked(currState) {
			// First entry into the interval: rt_last is this block's incoming
			// AppHash, the pre-interval state headers[0].prev_hash must bind
			// to; LastAnchoredHeight tracks the height it corresponds to, so
			// SubmitRecoveryProof can later enforce the proof's claimed
			// `count` against the real interval length.
			if err := k.LastAnchoredRoot.Set(ctx, types.ReduceToField(ctx.BlockHeader().AppHash)); err != nil {
				return err
			}
			if err := k.LastAnchoredHeight.Set(ctx, uint64(ctx.BlockHeight()-1)); err != nil {
				return err
			}
		}
		// The accumulator chains from the immediately preceding tracked
		// header, or from LastAnchoredRoot when HeaderHistory is empty --
		// either because this is the interval's first header, or because a
		// prior proof pruned the backlog up to the current tip while the
		// interval is still ongoing (pruneHeaderHistoryUpTo).
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
// (spec/core/EngramFSM.tla:294-301). Called from CommitFSMTransition, after
// this block's FSMState/HBtcAnchored writes above, so its own
// k.FSMState.Get/k.HBtcAnchored.Get reads pick up THIS block's freshly
// committed values, not last block's -- matching how every other certified
// value in this file already works (one block's CommitFSMTransition feeds
// the next block's PrepareProposal/ProcessProposal). It reads no live
// sensor, only already-committed keeper state, so it needs no vote-extension
// certification of its own.
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
