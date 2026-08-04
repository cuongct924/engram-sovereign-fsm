package vigilante

// Tolerance ports BTCTolerance (spec/core/EngramTendermint.tla:243-246):
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

// VerifySPVProof ports VerifySPVProof verbatim (spec/core/EngramTendermint.tla:271-275):
// simulates an SPV light client validating a BTC receipt. If an eclipsed
// proposer submits a forged branch (e.g. a "BTC_FORK" tag instead of
// "BTC_BLOCK"), the hash comparison rejects it.
//
//	VerifySPVProof(receipt) ==
//	    /\ receipt.checkpoint_block_height <= h_btc_current
//	    /\ receipt.checkpoint_block_height >= h_btc_anchored
//	    /\ receipt.checkpoint_block_hash = ExpectedBlockHash(receipt.checkpoint_block_height)
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
func VerifyReceipt(receipt Receipt, hBTCCurrent, hBTCAnchored, round uint64) bool {
	tol := Tolerance(round)
	lowerBound := int64(hBTCCurrent) - int64(tol)
	if lowerBound < 0 {
		lowerBound = 0
	}
	if receipt.CheckpointBlockHeight < uint64(lowerBound) {
		return false
	}
	return VerifySPVProof(receipt, hBTCCurrent, hBTCAnchored)
}
