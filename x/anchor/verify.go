package anchor

import (
	sovtypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// Tolerance ports BTCTolerance (spec/core/EngramTendermint.tla:243-246),
// widened with deliberate divergences:
//
// (1) kDeepFinality, not the spec's flat max of 1: a K-deep-confirmed anchor
// lags h_btc_current by >= kDeepFinality, so freshness (checkpoint >=
// h_btc_current - tol) and IsKDeep depth can never both hold if tol <
// kDeepFinality.
//
// (2) Widens from round 0, not round>=3: the spec's round<3 zero-tol case is
// unsatisfiable once (1) applies -- it would force a round-skip on every
// height regardless of real BTC health.
//
// (3) tol = kDeepFinality + livenessMargin, not exactly kDeepFinality: at
// equality the two bounds meet at one checkpoint height with zero slack for
// real gossip/round-skip delay. livenessMargin=2 matches AnchorTracker's
// kDeepFinality+1 requirement; an operational choice, not a spec constant.
//
// (4) +round, not a flat margin: h_btc_current keeps advancing while a stuck
// height round-skips, so a fixed tol has no recovery path once latency grows
// the gap past it. Growing tol with round mirrors BTCTolerance's own widen-
// under-contention, just unbounded instead of the abstract model's r>=3 -> 1 cap.
const livenessMargin = 2

func Tolerance(round, kDeepFinality uint64) uint64 {
	return kDeepFinality + livenessMargin + round
}

// VerifySPVProof ports VerifySPVProof verbatim (spec/core/EngramTendermint.tla:271-275):
//
//	VerifySPVProof(receipt) ==
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
func VerifySPVProof(receipt Receipt, hBTCCurrent, hBTCAnchored uint64) bool {
	if receipt.CheckpointBlockHeight > hBTCCurrent {
		return false
	}
	if receipt.CheckpointBlockHeight < hBTCAnchored {
		return false
	}
	return receipt.CheckpointBlockHash == ExpectedBlockHash(receipt.CheckpointBlockHeight)
}

// VerifyReceipt ports the "Settlement Monotonicity & BTC Light Client Hash
// Check" conjunct of IsValidProposal (spec/core/EngramTendermint.tla:296-312):
//
//	/\ (prop.fsm_state \in {"ANCHORED", "RECOVERING"} \/ IsBTCHealthy) =>
//	    /\ prop.btc_receipt.checkpoint_block_height >= (h_btc_current - btc_tol)
//	    /\ VerifySPVProof(prop.btc_receipt)
//
// The freshness/SPV checks apply only when the proposal claims ANCHORED/
// RECOVERING, or the BTC sensor is independently healthy -- otherwise (e.g.
// SOVEREIGN with BTC genuinely down) a stale/unverifiable checkpoint must not
// invalidate the proposal, or a real outage makes every proposal permanently
// unverifiable (E2 S6). Mirrors da.VerifyReceipt's requiresCheck gate.
func VerifyReceipt(receipt Receipt, fsmState string, isBTCHealthy bool, hBTCCurrent, hBTCAnchored, round, kDeepFinality uint64) bool {
	requiresCheck := fsmState == sovtypes.StateAnchored || fsmState == sovtypes.StateRecovering || isBTCHealthy
	if !requiresCheck {
		return true
	}

	tol := Tolerance(round, kDeepFinality)
	lowerBound := int64(hBTCCurrent) - int64(tol)
	if lowerBound < 0 {
		lowerBound = 0
	}
	if receipt.CheckpointBlockHeight < uint64(lowerBound) {
		return false
	}
	return VerifySPVProof(receipt, hBTCCurrent, hBTCAnchored)
}
