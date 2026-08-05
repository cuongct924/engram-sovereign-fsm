//go:build btcsmoke

package vigilante_test

// This is a manual, opt-in smoke test against a REAL running bitcoind
// regtest node (docker/bitcoin-regtest-cluster.yml's bitcoin-node01) --
// not part of the normal `go test ./...` suite (build tag gated), since CI/
// normal test runs have no bitcoind available. Run explicitly via:
//
//	go test -tags btcsmoke ./x/vigilante/... -run TestRPCClient_LiveSmoke -v

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cuongct220020/engram-sovereign-fsm/x/vigilante"
)

func TestRPCClient_LiveSmoke(t *testing.T) {
	c := vigilante.NewRPCClient("http://127.0.0.1:18443", "cuongct", "cuongct123")
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
	c := vigilante.NewRPCClient("http://127.0.0.1:18443", "cuongct", "cuongct123")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tracker := vigilante.NewAnchorTracker(c, 2) // kDeepFinality=2, small for a fast test

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

func mineBlocks(t *testing.T, ctx context.Context, c *vigilante.RPCClient, n int) string {
	t.Helper()
	addr, err := c.GetNewAddress(ctx)
	require.NoError(t, err)
	mineBlocksToAddr(t, ctx, c, n, addr)
	return addr
}

func mineBlocksToAddr(t *testing.T, ctx context.Context, c *vigilante.RPCClient, n int, addr string) {
	t.Helper()
	require.NoError(t, c.GenerateToAddress(ctx, n, addr))
}
