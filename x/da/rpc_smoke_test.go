//go:build celestiasmoke

package da_test

// Manual, opt-in smoke test against a REAL celestia-bridge, mirroring
// x/anchor/rpc_smoke_test.go's pattern. Reads its endpoint/token from the
// environment (fetched fresh per redeploy, unlike bitcoind's fixed creds).
//
//	CELESTIA_BRIDGE_URL=http://127.0.0.1:26658 \
//	CELESTIA_BRIDGE_AUTH_TOKEN=$(grep ^CELESTIA_BRIDGE_AUTH_TOKEN .env | cut -d= -f2) \
//	go test -tags celestiasmoke ./x/da/... -run LiveSmoke -v

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cuongct220020/engram-sovereign-fsm/x/da"
)

func liveClient(t *testing.T) *da.RPCClient {
	t.Helper()
	url := os.Getenv("CELESTIA_BRIDGE_URL")
	if url == "" {
		t.Skip("CELESTIA_BRIDGE_URL not set -- see this file's doc for how to run")
	}
	return da.NewRPCClient(url, os.Getenv("CELESTIA_BRIDGE_AUTH_TOKEN"))
}

func TestRPCClient_LiveSmoke(t *testing.T) {
	c := liveClient(t)
	require.True(t, c.Reachable(context.Background()), "bridge must be reachable at CELESTIA_BRIDGE_URL")

	ns, err := da.NewNamespace("smoketest0")
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	height, err := c.Submit(ctx, ns, []byte("engram-rpc-smoke"))
	require.NoError(t, err)
	t.Logf("submitted at celestia height: %d", height)

	// Submit already waits out Celestia's ~12s inclusion, but availability is
	// a separate RPC path -- poll briefly rather than assume one check.
	require.Eventually(t, func() bool {
		available, err := c.Available(ctx, height, ns)
		return err == nil && available
	}, 20*time.Second, time.Second, "submitted blob never became retrievable via blob.GetAll")
}

func TestPublisher_LiveSmoke(t *testing.T) {
	c := liveClient(t)
	ns, err := da.NewNamespace("smoketest0")
	require.NoError(t, err)
	p := da.NewPublisher(c, ns)

	require.True(t, p.ProbeHealthy(context.Background()))

	require.NoError(t, p.MaybePublish(context.Background(), 999, []byte("engram-publisher-smoke")))

	// Submit runs in a background goroutine; a second call checks Available()
	// against the pending submission and marks it verified once retrievable.
	require.Eventually(t, func() bool {
		require.NoError(t, p.MaybePublish(context.Background(), 999, nil))
		_, ok := p.VerifiedHeight()
		return ok
	}, 30*time.Second, 2*time.Second, "Publisher never marked height 999 verified")

	height, ok := p.VerifiedHeight()
	require.True(t, ok)
	require.Equal(t, uint64(999), height)
	require.False(t, p.Failed())
}
