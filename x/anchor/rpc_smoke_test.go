//go:build btcsmoke

package anchor_test

// This is a manual, opt-in smoke test against a REAL running bitcoind
// regtest node (docker/bitcoin-regtest-cluster.yml's bitcoin-node01) --
// not part of the normal `go test ./...` suite (build tag gated), since CI/
// normal test runs have no bitcoind available. Run explicitly via:
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

// TestAnchorTracker_ConfirmedHeightAlwaysPassesVerifyAnchor exercises the
// EXACT confirmation-count boundary the earlier TestAnchorTracker_LiveSmoke
// mines past without noticing: at exactly kDeepFinality (2) confirmations
// (not more), MaybeSubmit must NOT yet report confirmed -- and the first
// height it DOES report confirmed at must always pass VerifyAnchor
// immediately, with no extra confirmation needed. Found live: an earlier
// version of AnchorTracker.MaybeSubmit required only `confirmations >=
// kDeepFinality`, but VerifyAnchor (and every other validator's independent
// ProcessProposal re-check) implements the spec's IsKDeep with one fewer
// block of slack (h_btc_current - height >= kDeepFinality, no +1) --
// bitcoind's inclusive confirmations field made these off by exactly one, so
// every claimed anchor advance was rejected by every validator, 100% of the
// time, on the real 4-node testnet.
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

// TestRPCClient_ConcurrentSubmitOpReturnDoesNotRaceOnSharedUTXOs reproduces
// the real 4-validator scenario (this repo's 4 validators share one
// bitcoind wallet, x-engram-node-env's BITCOIN_HOST) by firing 4 concurrent
// SubmitOpReturn calls against the same wallet. Found live: an earlier
// version called fundrawtransaction (selects UTXOs, unlocked), THEN
// decoderawtransaction, THEN a separate lockunspent -- a real TOCTOU window
// where two concurrent calls could both select the same input before either
// locked it, failing with "lockunspent: -8 Invalid parameter, output
// already locked" and, on the real 4-node testnet, NEVER anchoring at all
// (every validator's submission raced every block, forever). The fix passes
// lockUnspents to fundrawtransaction itself so selection and locking happen
// as one atomic RPC call -- every concurrent call here must either succeed
// outright or fail with a benign "insufficient funds"/"already locked from
// a DIFFERENT already-broadcast tx" reason, never the TOCTOU race itself.
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
