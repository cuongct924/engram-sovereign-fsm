package da

import (
	sovtypes "github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// Tolerance ports DATolerance (spec/core/EngramTendermint.tla:237-241):
// dynamic tolerance widening the DA-receipt freshness window as consensus
// rounds accumulate (network is presumed to be under stress).
//
//	DATolerance(r) ==
//	    CASE r <= 1 -> 0
//	      [] r = 2  -> 2
//	      [] r >= 3 -> 4
//	      [] OTHER  -> 0
func Tolerance(round uint64) uint64 {
	switch {
	case round <= 1:
		return 0
	case round == 2:
		return 2
	default: // round >= 3
		return 4
	}
}

// VerifyReceipt ports the "DA Pipeline Check" conjunct of IsValidProposal
// (spec/core/EngramTendermint.tla:290-294):
//
//	/\ (prop.fsm_state \in {"ANCHORED", "RECOVERING"} \/ IsDAHealthy) =>
//	    /\ prop.da_receipt.attestation = TRUE
//	    /\ prop.da_receipt.published_block_height <= h_engram_current
//	    /\ prop.da_receipt.published_block_height >= (h_engram_current - DA_THRESHOLD - da_tol)
//
// The freshness checks only apply when the proposal claims ANCHORED/RECOVERING,
// or when the DA sensor is independently healthy -- otherwise (e.g. SOVEREIGN
// with DA genuinely down) a stale/absent receipt does not itself invalidate
// the proposal.
func VerifyReceipt(receipt Receipt, fsmState string, isDAHealthy bool, hEngramCurrent, daThreshold uint64, round uint64) bool {
	requiresCheck := fsmState == sovtypes.StateAnchored || fsmState == sovtypes.StateRecovering || isDAHealthy
	if !requiresCheck {
		return true
	}

	if !receipt.Attestation {
		return false
	}
	if receipt.PublishedBlockHeight > hEngramCurrent {
		return false
	}

	tol := Tolerance(round)
	lowerBound := int64(hEngramCurrent) - int64(daThreshold) - int64(tol)
	if lowerBound < 0 {
		lowerBound = 0
	}
	return receipt.PublishedBlockHeight >= uint64(lowerBound)
}
