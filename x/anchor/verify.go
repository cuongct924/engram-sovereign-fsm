package anchor

// Tolerance ports BTCTolerance's shape (spec/core/EngramTendermint.tla:243-246:
// CASE r<=2 -> 0, r>=3 -> 1, OTHER -> 0), widened with three deliberate
// divergences from the literal formula:
//
// (1) kDeepFinality, not the spec's flat max of 1: a real K-deep-confirmed
// anchor (AnchorTracker) always lags h_btc_current by >= kDeepFinality
// (IsKDeep's own safety requirement), so VerifyReceipt's freshness bound
// (checkpoint >= h_btc_current - tol) and IsKDeep's depth bound
// (h_btc_current - checkpoint >= kDeepFinality) can never both hold if
// tol < kDeepFinality.
//
// (2) Widens from round 0, not just round>=3: the spec's round<3
// zero-tolerance case is structurally unsatisfiable once (1) applies --
// exact freshness against a K-deep-lagging anchor never holds -- so keeping
// it would just force a guaranteed round-skip on every height regardless of
// real BTC health.
//
// (3) tol = kDeepFinality + livenessMargin, not exactly kDeepFinality: at
// tol == kDeepFinality, freshness and depth bounds meet at exactly one
// checkpoint height with zero slack, which any real gossip/round-skip delay
// between confirmation and proposal acceptance pushes h_btc_current past.
// livenessMargin=2 (matching AnchorTracker's own kDeepFinality+1
// confirmation requirement) is an operational choice for this testnet's
// cadence, not a spec constant -- revisit if round-trip latency changes materially.
const livenessMargin = 2

func Tolerance(_, kDeepFinality uint64) uint64 {
	return kDeepFinality + livenessMargin
}

// VerifySPVProof ports VerifySPVProof verbatim (spec/core/EngramTendermint.tla:271-275):
//
//	VerifySPVProof(receipt) ==
//	    /\ receipt.checkpoint_block_height <= h_btc_current
//	    /\ receipt.checkpoint_block_height >= h_btc_anchored
//	    /\ receipt.checkpoint_block_hash = ExpectedBlockHash(receipt.checkpoint_block_height)
//
// The `>= h_btc_anchored` conjunct is the concrete counterpart of
// EngramConsensus.tla's IsKDeep (part of CanElect's ANCHORED/SUSPICIOUS
// branch, M0c) -- CanElect itself has no separate concrete implementation:
// EngramServerRefinement.tla only uses it inside the refinement-proof
// INSTANCE substitution, never at runtime. This function is the real code
// that makes IsKDeep hold concretely (see verify_test.go's
// TestVerifySPVProof_RejectsCheckpointBelowAnchored).
//
// CanElect's SOVEREIGN-state IsMaxStakeBranch has no comparable artifact:
// it requires SumStake >= TOTAL_STAKE/2, weaker than CometBFT's own
// unmodified >=2/3 commit quorum, so any block the fork actually commits
// already satisfies it structurally.
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
// Check" conjunct of IsValidProposal (spec/core/EngramTendermint.tla:296-298):
//
//	/\ prop.btc_receipt.checkpoint_block_height >= (h_btc_current - btc_tol)
//	/\ VerifySPVProof(prop.btc_receipt)
func VerifyReceipt(receipt Receipt, hBTCCurrent, hBTCAnchored, round, kDeepFinality uint64) bool {
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
