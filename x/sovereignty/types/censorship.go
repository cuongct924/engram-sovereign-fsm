package types

// IsCensoring mirrors IsCensoring (spec/core/EngramTendermint.tla:310-315):
// true iff some forced tx has already been ignored for >= maxIgnoreRounds
// rounds AND is still absent from the current proposal. Keys of
// ignoredRounds/includedInProposal are raw tx byte content (matching
// forced_tx_queue's identifier convention, see x/sovereignty/proposal.go's
// containsWithdrawal for the same raw-byte-marker approach).
func IsCensoring(forcedTxQueue []string, ignoredRounds map[string]uint64, includedInProposal map[string]bool, maxIgnoreRounds uint64) bool {
	for _, tx := range forcedTxQueue {
		if ignoredRounds[tx] >= maxIgnoreRounds && !includedInProposal[tx] {
			return true
		}
	}
	return false
}

// NextIgnoredRounds mirrors UpdateIgnoredRounds (spec/core/EngramTendermint.tla:493-503):
// resets a forced tx's ignored-round counter to 0 if it appeared in the
// just-committed block, otherwise increments it, capped at maxIgnoreRounds+1
// (the spec's MinVal(_, MAX_IGNORE_ROUNDS + 1)).
func NextIgnoredRounds(forcedTxQueue []string, current map[string]uint64, includedInBlock map[string]bool, maxIgnoreRounds uint64) map[string]uint64 {
	next := make(map[string]uint64, len(forcedTxQueue))
	for _, tx := range forcedTxQueue {
		if includedInBlock[tx] {
			next[tx] = 0
			continue
		}
		count := current[tx] + 1
		if cap := maxIgnoreRounds + 1; count > cap {
			count = cap
		}
		next[tx] = count
	}
	return next
}
