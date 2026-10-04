package anchor

import (
	sovtypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// Tolerance ports BTCTolerance (spec/core/EngramTendermint.tla:272-275):
// the round-dependent widening of the BTC freshness window, capped at r >= 3.
//
//	BTCTolerance(r) ==
//	    CASE r <= 2 -> 0
//	      [] r >= 3 -> 1
//	      [] OTHER  -> 0
func Tolerance(round uint64) uint64 {
	if round >= 3 {
		return 1
	}
	return 0
}

// livenessMargin is the slack on top of kDeepFinality in Threshold. At
// equality the freshness and IsKDeep bounds meet at one checkpoint height
// with zero slack for real gossip/round-skip delay; 2 matches AnchorTracker's
// kDeepFinality+1 requirement. An operational choice, not a spec constant.
const livenessMargin = 2

// Threshold is BTC_THRESHOLD (spec/core/EngramTendermint.tla:62-69), the BTC
// analogue of DA_THRESHOLD. Deliberate divergence from the spec's abstract
// model: a K-deep-confirmed anchor lags h_btc_current by >= kDeepFinality, so
// the base staleness cannot be below it or freshness and IsKDeep never both
// hold. Replaces the old unbounded round-widening; Tolerance carries the cap.
func Threshold(kDeepFinality uint64) uint64 {
	return kDeepFinality + livenessMargin
}

// VerifyAnchorReceipt ports VerifyAnchorReceipt verbatim (spec/core/EngramTendermint.tla:271-275):
//
//	VerifyAnchorReceipt(receipt) ==
//	    /\ receipt.checkpoint_block_height <= h_btc_current
//	    /\ receipt.checkpoint_block_height >= h_btc_anchored
//	    /\ receipt.checkpoint_block_hash = ExpectedBlockHash(receipt.checkpoint_block_height)
//
// The `>= h_btc_anchored` conjunct is the concrete counterpart of IsKDeep
// (CanElect's ANCHORED/SUSPICIOUS branch) -- CanElect itself has no concrete
// implementation (refinement-proof INSTANCE only, never runtime). This is the
// code that makes IsKDeep hold concretely (see verify_test.go).
//
// CanElect's SOVEREIGN-state IsMaxStakeBranch has no comparable artifact: it
// needs SumStake >= TOTAL_STAKE/2, weaker than CometBFT's unmodified >=2/3
// commit quorum, so any block the fork commits already satisfies it.
func VerifyAnchorReceipt(receipt Receipt, hBTCCurrent, hBTCAnchored uint64) bool {
	if receipt.CheckpointBlockHeight > hBTCCurrent {
		return false
	}
	if receipt.CheckpointBlockHeight < hBTCAnchored {
		return false
	}
	return receipt.CheckpointBlockHash == ExpectedBlockHash(receipt.CheckpointBlockHeight)
}

// VerifyReceipt ports the "Settlement Monotonicity & BTC Light Client Hash
// Check" conjunct of IsValidProposal (spec/core/EngramTendermint.tla:353-367):
//
//	/\ (prop.fsm_state \in {"ANCHORED", "RECOVERING"} \/ IsBTCHealthy) =>
//	    /\ prop.btc_receipt.checkpoint_block_height >= (h_btc_current - BTC_THRESHOLD - btc_tol)
//	    /\ VerifyAnchorReceipt(prop.btc_receipt)
//
// The freshness/anchor-hash checks apply only when the proposal claims ANCHORED/
// RECOVERING, or the BTC sensor is independently healthy -- otherwise (e.g.
// SOVEREIGN with BTC genuinely down) a stale/unverifiable checkpoint must not
// invalidate the proposal, or a real outage makes every proposal permanently
// unverifiable (E2 S6). Mirrors da.VerifyReceipt's requiresCheck gate.
func VerifyReceipt(receipt Receipt, fsmState string, isBTCHealthy bool, hBTCCurrent, hBTCAnchored, round, kDeepFinality uint64) bool {
	requiresCheck := fsmState == sovtypes.StateAnchored || fsmState == sovtypes.StateRecovering || isBTCHealthy
	if !requiresCheck {
		return true
	}

	lowerBound := int64(hBTCCurrent) - int64(Threshold(kDeepFinality)) - int64(Tolerance(round))
	if lowerBound < 0 {
		lowerBound = 0
	}
	if receipt.CheckpointBlockHeight < uint64(lowerBound) {
		return false
	}
	return VerifyAnchorReceipt(receipt, hBTCCurrent, hBTCAnchored)
}
