package vigilante

// Tolerance ports BTCTolerance's shape (spec/core/EngramTendermint.tla:243-246)
// with one deliberate divergence from its literal formula:
//
//	BTCTolerance(r) ==
//	    CASE r <= 2 -> 0
//	      [] r >= 3 -> 1
//	      [] OTHER  -> 0
//
// The spec's flat max (1) assumes checkpoint submission and h_btc_current
// stay tightly coupled -- reasonable in the abstract model, but structurally
// incompatible with a REAL K-deep-confirmed anchor (Phase 7's
// vigilante.AnchorTracker): such an anchor, by definition, lags
// h_btc_current by at least kDeepFinality blocks (IsKDeep's safety
// requirement), so VerifyReceipt's freshness requirement
// (checkpoint_height >= h_btc_current - tol) and IsKDeep's safety
// requirement (h_btc_current - checkpoint_height >= kDeepFinality) can never
// BOTH hold if tol < kDeepFinality -- found by running a real node against
// real bitcoind, where every proposal was rejected forever once
// h_btc_current started advancing from live data. Widening the r>=3
// tolerance to exactly kDeepFinality (instead of the spec's flat 1) closes
// that gap: once enough rounds have elapsed for a real K-deep confirmation
// to plausibly exist, the freshness window is exactly wide enough to accept
// one, no wider.
func Tolerance(round, kDeepFinality uint64) uint64 {
	if round >= 3 {
		return kDeepFinality
	}
	return 0
}

// VerifySPVProof ports VerifySPVProof verbatim (spec/core/EngramTendermint.tla:271-275):
// simulates an SPV light client validating a BTC receipt. If an eclipsed
// proposer submits a forged branch (e.g. a "BTC_FORK" tag instead of
// "BTC_BLOCK"), the hash comparison rejects it.
//
//	VerifySPVProof(receipt) ==
//	    /\ receipt.checkpoint_block_height <= h_btc_current
//	    /\ receipt.checkpoint_block_height >= h_btc_anchored
//	    /\ receipt.checkpoint_block_hash = ExpectedBlockHash(receipt.checkpoint_block_height)
//
// The `>= h_btc_anchored` conjunct is this function's concrete counterpart of
// EngramConsensus.tla's IsKDeep -- "c.btc_anchored <= h_btc_anchored (not
// lost to reorg)" -- which is itself one branch of CanElect (M0c,
// EngramConsensus.tla:143-149), the abstract LiDO fork-choice rule for
// ANCHORED/SUSPICIOUS state. CanElect has no separate concrete
// implementation obligation in the CometBFT fork: EngramServerRefinement.tla
// only ever uses CanElect/IsKDeep/IsMaxStakeBranch inside the `INSTANCE ...
// WITH` substitution that feeds concrete state into the abstract layer for
// the refinement PROOF -- ServerInsertProposal (the concrete hook mapped to
// abstract Pull, EngramServer.tla:52) never calls them at runtime. This
// function -- already wired into ProcessProposal via VerifyReceipt below --
// is the real, executing code that makes IsKDeep hold concretely; see
// verify_test.go's TestVerifySPVProof_RejectsCheckpointBelowAnchored for the
// direct test coverage. (The SOVEREIGN-state IsMaxStakeBranch branch of
// CanElect has no comparable app-layer artifact to test: it requires
// SumStake[voters] >= TOTAL_STAKE/2, a WEAKER bound than CometBFT's own
// unmodified >=2/3 commit quorum, so any block the fork's vanilla vote-
// counting actually commits already satisfies it structurally -- this repo
// deliberately does not reimplement CometBFT's quorum math at the app layer,
// per "sensors propose, consensus decides" in CLAUDE.md.)
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
//
// kDeepFinality is Tolerance's widened bound for round>=3 -- see its doc for
// why the spec's flat max no longer suffices once a real AnchorTracker is
// wired in.
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
