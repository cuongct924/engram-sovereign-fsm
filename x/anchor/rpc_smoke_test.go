//go:build btcsmoke

package anchor_test

// Manual, opt-in smoke test against a REAL running bitcoind regtest node
// (docker/bitcoin-regtest-cluster.yml's bitcoin-node01) -- build-tag gated
// out of normal test runs (no bitcoind in CI). Run explicitly:
//
//	go test -tags btcsmoke ./x/anchor/... -run TestRPCClient_LiveSmoke -v

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cuongct220020/engram-sovereign-fsm/x/anchor"
)

func TestRPCClient_LiveSmoke(t *testing.T) {
	c := anchor.NewRPCClient("http://127.0.0.1:18443", "cuongct", "cuongct123")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	height, err := c.CurrentHeight(ctx)
	require.NoError(t, err)
	t.Logf("live regtest height: %d", height)

	hash, err := c.BlockHashAt(ctx, 0)
	require.NoError(t, err)
	t.Logf("genesis hash: %s", hash)
	require.Len(t, hash, 64, "block hash must be 32 bytes hex-encoded")
}

func TestAnchorTracker_LiveSmoke(t *testing.T) {
	c := anchor.NewRPCClient("http://127.0.0.1:18443", "cuongct", "cuongct123")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tracker := anchor.NewAnchorTracker(c, 2) // kDeepFinality=2, small for a fast test

	require.NoError(t, tracker.MaybeSubmit(ctx, 42))
	_, ok := tracker.ConfirmedAnchorHeight()
	require.False(t, ok, "must not be confirmed before any block mines it")

	addr := mineBlocks(t, ctx, c, 1)
	require.NoError(t, tracker.MaybeSubmit(ctx, 42)) // 1 confirmation, still < kDeepFinality=2
	_, ok = tracker.ConfirmedAnchorHeight()
	require.False(t, ok, "must not be confirmed at only 1 of 2 required confirmations")

	mineBlocksToAddr(t, ctx, c, 2, addr)
	require.NoError(t, tracker.MaybeSubmit(ctx, 43))
	height, ok := tracker.ConfirmedAnchorHeight()
	require.True(t, ok, "must be confirmed once kDeepFinality=2 is reached")
	t.Logf("confirmed anchor height: %d", height)

	verified, err := tracker.VerifyAnchor(ctx, height)
	require.NoError(t, err)
	require.True(t, verified, "an independent tracker must be able to verify our own confirmed anchor")

	verifiedWrong, err := tracker.VerifyAnchor(ctx, height-1)
	require.NoError(t, err)
	require.False(t, verifiedWrong, "must not verify a height that doesn't actually carry our tag")
}

// Exercises the exact confirmation-count boundary LiveSmoke mines past: at
// exactly kDeepFinality confirmations MaybeSubmit must NOT yet confirm, and
// the first height it DOES confirm at must pass VerifyAnchor immediately.
// Regression for the off-by-one found live: MaybeSubmit required
// `confirmations >= kDeepFinality` but VerifyAnchor's spec IsKDeep has no
// +1 slack (h_btc_current - height >= kDeepFinality), and bitcoind's
// confirmations field is inclusive -- so every claimed anchor advance was
// rejected by every validator on the real 4-node testnet.
func TestAnchorTracker_ConfirmedHeightAlwaysPassesVerifyAnchor(t *testing.T) {
	c := anchor.NewRPCClient("http://127.0.0.1:18443", "cuongct", "cuongct123")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tracker := anchor.NewAnchorTracker(c, 2) // kDeepFinality=2
	require.NoError(t, tracker.MaybeSubmit(ctx, 1000))

	addr := mineBlocks(t, ctx, c, 2) // exactly kDeepFinality confirmations, no more
	require.NoError(t, tracker.MaybeSubmit(ctx, 1000))
	_, ok := tracker.ConfirmedAnchorHeight()
	require.False(t, ok, "must NOT be confirmed at exactly kDeepFinality confirmations -- needs kDeepFinality+1")

	mineBlocksToAddr(t, ctx, c, 1, addr) // one more -> kDeepFinality+1 total
	require.NoError(t, tracker.MaybeSubmit(ctx, 1000))
	height, ok := tracker.ConfirmedAnchorHeight()
	require.True(t, ok, "must be confirmed at kDeepFinality+1 confirmations")

	verified, err := tracker.VerifyAnchor(ctx, height)
	require.NoError(t, err)
	require.True(t, verified, "the height MaybeSubmit just reported confirmed must immediately pass VerifyAnchor -- this is exactly what failed live before the fix")
}

// Reproduces the real 4-validator scenario (all 4 share one bitcoind
// wallet) by firing 4 concurrent SubmitOpReturn calls at the same wallet.
// Regression for the TOCTOU found live: an earlier version selected UTXOs
// in fundrawtransaction and only locked them in a separate lockunspent, so
// concurrent calls could both select the same input and, on the real
// testnet, never anchor at all. The fix passes lockUnspents to
// fundrawtransaction (selection+lock become one atomic RPC); concurrent
// calls here must succeed or fail benignly, never with the race itself.
func TestRPCClient_ConcurrentSubmitOpReturnDoesNotRaceOnSharedUTXOs(t *testing.T) {
	c := anchor.NewRPCClient("http://127.0.0.1:18443", "cuongct", "cuongct123")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	mineBlocks(t, ctx, c, 5) // ensure enough spendable, mature UTXOs for 4 concurrent funds

	const n = 4
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = c.SubmitOpReturn(ctx, []byte{byte(i)})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err == nil {
			continue
		}
		require.NotContains(t, err.Error(), "already locked",
			"validator %d hit the UTXO-selection race the atomic lockUnspents fix should prevent: %v", i, err)
	}
}

func mineBlocks(t *testing.T, ctx context.Context, c *anchor.RPCClient, n int) string {
	t.Helper()
	addr, err := c.GetNewAddress(ctx)
	require.NoError(t, err)
	mineBlocksToAddr(t, ctx, c, n, addr)
	return addr
}

func mineBlocksToAddr(t *testing.T, ctx context.Context, c *anchor.RPCClient, n int, addr string) {
	t.Helper()
	require.NoError(t, c.GenerateToAddress(ctx, n, addr))
}
