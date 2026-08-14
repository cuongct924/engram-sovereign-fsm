package types

// IsCensoring mirrors IsCensoring (spec/core/EngramTendermint.tla:310-315):
// true iff some forced tx has been ignored >= maxIgnoreRounds rounds and is
// still absent from the current proposal.
func IsCensoring(forcedTxQueue []string, ignoredRounds map[string]uint64, includedInProposal map[string]bool, maxIgnoreRounds uint64) bool {
	for _, tx := range forcedTxQueue {
		if ignoredRounds[tx] >= maxIgnoreRounds && !includedInProposal[tx] {
			return true
		}
	}
	return false
}

// NextIgnoredRounds mirrors UpdateIgnoredRounds
// (spec/core/EngramTendermint.tla:493-503): a forced tx's counter resets to
// 0 if it appeared in the committed block, else increments, capped at
// maxIgnoreRounds+1 (the spec's MinVal(_, MAX_IGNORE_ROUNDS + 1)).
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
