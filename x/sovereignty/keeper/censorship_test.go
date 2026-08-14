package keeper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections/colltest"
)

func newTestKeeper(t *testing.T) (*Keeper, context.Context) {
	t.Helper()
	storeService, ctx := colltest.MockStore()
	return NewKeeper(storeService, nil), ctx
}

func TestForcedTxQueueSlice(t *testing.T) {
	k, ctx := newTestKeeper(t)

	t.Run("empty queue returns empty slice", func(t *testing.T) {
		txs, err := k.ForcedTxQueueSlice(ctx)
		require.NoError(t, err)
		require.Empty(t, txs)
	})

	t.Run("returns all queued txs", func(t *testing.T) {
		for _, tx := range []string{"a", "b", "c"} {
			require.NoError(t, k.ForcedTxQueue.Set(ctx, tx))
		}
		txs, err := k.ForcedTxQueueSlice(ctx)
		require.NoError(t, err)
		require.ElementsMatch(t, []string{"a", "b", "c"}, txs)
	})
}

func TestIgnoredRoundsMap(t *testing.T) {
	k, ctx := newTestKeeper(t)

	require.NoError(t, k.TxIgnoredRounds.Set(ctx, "a", 3))
	require.NoError(t, k.TxIgnoredRounds.Set(ctx, "b", 1))

	t.Run("missing tx defaults to zero", func(t *testing.T) {
		m := k.IgnoredRoundsMap(ctx, []string{"nope"})
		require.Equal(t, uint64(0), m["nope"])
	})

	t.Run("existing counters are preserved", func(t *testing.T) {
		m := k.IgnoredRoundsMap(ctx, []string{"a", "b"})
		require.Equal(t, uint64(3), m["a"])
		require.Equal(t, uint64(1), m["b"])
	})
}
