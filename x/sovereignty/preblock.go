package sovereignty

import (
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/keeper"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// NewPreBlocker builds the sdk.PreBlocker for this module, mirroring
// ServerUponProposalInPrecommitNoDecision (spec/core/EngramServer.tla:135-189):
// the FSM state committed here is the ALREADY-AGREED fsm_state embedded in the
// decided block's extended proposal (Txs[0]) -- it is never recomputed
// locally at this point, matching the "sensors propose, consensus decides"
// separation the spec depends on (see CLAUDE.md's FSM-layer notes). Wiring
// this onto a real BaseApp via SetPreBlocker is M5's job -- this function
// only builds the handler.
func NewPreBlocker(k *keeper.Keeper) sdk.PreBlocker {
	return func(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
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
// run once per finalized block -- the natural real-chain analogue of "once
// per round transition" the spec uses, since vanilla ABCI 2.0 doesn't expose
// round number to app hooks (see NewProcessProposalHandler's censorship-check
// comment for the same gap). No-ops when forced_tx_queue is empty.
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
		if err := k.TxIgnoredRounds.Set(ctx, tx, count); err != nil {
			return err
		}
	}
	return nil
}

// CommitFSMTransition ports ServerUponProposalInPrecommitNoDecision's state
// writes (spec/core/EngramServer.tla:135-189, steps 3-5; steps 1-2 --
// extracting the decided proposal -- already happened via DecodeExtendedProposal).
//
// Note: the spec's step 5 also clears is_btc_spv_failed, which this Go port's
// PeripheralMetrics has no equivalent field for (only is_das_failed /
// is_attestation_failed were ported from EngramFSM.tla's networkSensorVars in
// Phase 1) -- documented gap, not a silent omission.
func CommitFSMTransition(ctx sdk.Context, k *keeper.Keeper, ext ExtendedProposal) error {
	currState, err := k.FSMState.Get(ctx)
	if err != nil {
		currState = types.StateAnchored
	}
	safeBlocks, _ := k.SafeBlocks.Get(ctx)
	suspiciousDuration, _ := k.SuspiciousDuration.Get(ctx)

	// Step 3: drive the FSM transition to the agreed state and update anchored heights.
	if err := k.FSMState.Set(ctx, ext.FSMState); err != nil {
		return err
	}
	if err := k.SafeBlocks.Set(ctx, keeper.NextSafeBlocks(currState, ext.FSMState, safeBlocks, k.Params)); err != nil {
		return err
	}
	if err := k.SuspiciousDuration.Set(ctx, keeper.NextSuspiciousDuration(currState, ext.FSMState, suspiciousDuration, k.Params)); err != nil {
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
	if ext.FSMState == types.StateRecovering && ext.ZKProofRef {
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

	// Step 5b: a real ZK proof (keeper.MsgServerImpl.SubmitRecoveryProof)
	// only ever applies to the SOVEREIGN/RECOVERING interval it was proven
	// against. Once RECOVERING is left -- whether successfully to ANCHORED
	// or back down to SOVEREIGN on a fresh setback -- that latch is stale:
	// a NEW interval (if any) needs a NEW proof. See
	// sensors_refresh.go's refreshReanchoringProofValid, the only reader.
	if currState == types.StateRecovering && ext.FSMState != types.StateRecovering {
		if err := k.RealProofSubmittedHeight.Set(ctx, 0); err != nil {
			return err
		}
	}

	// Step 6: track real per-block header history for the ZK re-anchoring
	// circuit's witness (spec/README.md's §Re-anchoring via ZK-Proof of
	// Recovery; circuit/reanchoring/src/main.nr's Header{fsm_state,
	// withdrawal_locked, state_root}). Only meaningful while the FSM is
	// SOVEREIGN/RECOVERING -- see types.RecoveryHeader's doc for why
	// StateRoot is CometBFT's real AppHash rather than the keeper's (dead,
	// unwired) SMT.
	if types.WithdrawLocked(ext.FSMState) {
		if !types.WithdrawLocked(currState) {
			// Entering the interval for the first time: rt_last (README:
			// "the state root of the last Bitcoin-anchored block") is this
			// block's incoming AppHash -- the state right before the
			// interval starts, which headers[0].prev_hash must bind to.
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
		// Interval just ended (back to ANCHORED): this history is no
		// longer needed for a proof -- a future interval starts its own
		// LastAnchoredRoot from scratch above.
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
