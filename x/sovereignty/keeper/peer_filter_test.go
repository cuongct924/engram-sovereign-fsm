package keeper

import (
	"testing"

	"github.com/stretchr/testify/require"

	"cosmossdk.io/collections/colltest"
)

// fakePeerFilterSource is a test double for PeerFilterSource -- reports a
// fixed count regardless of which subnet is queried, enough to exercise
// FilterPeerByAddr's boundary logic without a real *p2p.Switch.
type fakePeerFilterSource struct {
	count uint64
}

func (f fakePeerFilterSource) PeerCountInSubnet(string) uint64 {
	return f.count
}

func newPeerFilterTestKeeper(t *testing.T, maxPerSubnet uint64) *Keeper {
	t.Helper()
	storeService, _ := colltest.MockStore()
	k := NewKeeper(storeService, nil)
	k.Params.MaxPeersPerSubnet = maxPerSubnet
	return k
}

// TestFilterPeerByAddr_FailsOpenWithNoSource covers the cold-start window
// (BaseApp registers FilterPeerByAddr before node.NewNode() constructs the
// real Switch) -- must accept, never reject, before SetPeerFilterSource.
func TestFilterPeerByAddr_FailsOpenWithNoSource(t *testing.T) {
	k := newPeerFilterTestKeeper(t, 2)
	resp := k.FilterPeerByAddr("10.0.0.5:12345")
	require.Zero(t, resp.Code, "must accept when no PeerFilterSource is wired yet")
}

// TestFilterPeerByAddr_FailsOpenOnUnparseableAddr covers a malformed info
// string (never expected from a real net.Conn.RemoteAddr().String(), but
// this is best-effort health parsing, not consensus-critical).
func TestFilterPeerByAddr_FailsOpenOnUnparseableAddr(t *testing.T) {
	k := newPeerFilterTestKeeper(t, 2)
	k.SetPeerFilterSource(fakePeerFilterSource{count: 999})
	resp := k.FilterPeerByAddr("not-an-address")
	require.Zero(t, resp.Code, "must accept on unparseable info rather than erroring")
}

// TestFilterPeerByAddr_AcceptsBelowLimit is the gap=1-under boundary: a
// subnet at exactly MaxPeersPerSubnet-1 connected peers must still admit one
// more.
func TestFilterPeerByAddr_AcceptsBelowLimit(t *testing.T) {
	k := newPeerFilterTestKeeper(t, 4)
	k.SetPeerFilterSource(fakePeerFilterSource{count: 3})
	resp := k.FilterPeerByAddr("172.28.0.9:26656")
	require.Zero(t, resp.Code, "count below MaxPeersPerSubnet must be accepted")
}

// TestFilterPeerByAddr_RejectsAtLimit confirms the bound is actually
// enforced -- a subnet already AT MaxPeersPerSubnet must reject the next
// connection attempt, not silently admit it.
func TestFilterPeerByAddr_RejectsAtLimit(t *testing.T) {
	k := newPeerFilterTestKeeper(t, 4)
	k.SetPeerFilterSource(fakePeerFilterSource{count: 4})
	resp := k.FilterPeerByAddr("172.28.0.9:26656")
	require.NotZero(t, resp.Code, "count at MaxPeersPerSubnet must be rejected")
}

// TestFilterPeerByAddr_RejectsAboveLimit is the same check further past the
// bound, guarding against an off-by-one the other direction.
func TestFilterPeerByAddr_RejectsAboveLimit(t *testing.T) {
	k := newPeerFilterTestKeeper(t, 4)
	k.SetPeerFilterSource(fakePeerFilterSource{count: 10})
	resp := k.FilterPeerByAddr("172.28.0.9:26656")
	require.NotZero(t, resp.Code, "count well above MaxPeersPerSubnet must be rejected")
}

// TestFilterPeerByAddr_HandlesBareIPWithoutPort covers a fallback path
// (SplitHostPort failing, e.g. no port present) -- ParseIP must still
// succeed against the raw info string.
func TestFilterPeerByAddr_HandlesBareIPWithoutPort(t *testing.T) {
	k := newPeerFilterTestKeeper(t, 4)
	k.SetPeerFilterSource(fakePeerFilterSource{count: 4})
	resp := k.FilterPeerByAddr("172.28.0.9")
	require.NotZero(t, resp.Code, "bare IP (no port) must still be parsed and subject to the same limit")
}
