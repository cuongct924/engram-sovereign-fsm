package keeper

import "context"

// ForcedTxQueueSlice reads the full forced_tx_queue into a slice for use with
// types.IsCensoring/NextIgnoredRounds (spec/core/EngramTendermint.tla's
// forced_tx_queue, a set in the spec, is stored as a collections.KeySet here).
func (k *Keeper) ForcedTxQueueSlice(ctx context.Context) ([]string, error) {
	var txs []string
	err := k.ForcedTxQueue.Walk(ctx, nil, func(tx string) (stop bool, err error) {
		txs = append(txs, tx)
		return false, nil
	})
	return txs, err
}

// IgnoredRoundsMap reads tx_ignored_rounds for exactly the given forced txs
// (a targeted lookup, not a full-table scan, since callers already hold the
// queue). A tx with no stored counter defaults to 0, matching the spec's
// FSMInit-style zero-initialization.
func (k *Keeper) IgnoredRoundsMap(ctx context.Context, forcedTxQueue []string) map[string]uint64 {
	m := make(map[string]uint64, len(forcedTxQueue))
	for _, tx := range forcedTxQueue {
		count, err := k.TxIgnoredRounds.Get(ctx, tx)
		if err != nil {
			count = 0
		}
		m[tx] = count
	}
	return m
}
