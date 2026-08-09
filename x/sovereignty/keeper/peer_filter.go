package keeper

import (
	"net"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// PeerFilterSource abstracts a live count of currently-connected peers per
// subnet -- concretely cmd/engramd's vanillaP2PHealthAdapter, wired in via
// SetPeerFilterSource. Mirrors sensors.P2PHealthSource's SetSource pattern
// (x/sovereignty/keeper/sensors/p2p_health.go): this package intentionally
// does not import the CometBFT fork's p2p package directly, only cmd/engramd
// does, so the adapter lives at that wiring layer.
type PeerFilterSource interface {
	PeerCountInSubnet(subnet string) uint64
}

// SetPeerFilterSource wires src in. Until called, FilterPeerByAddr fails
// open (accepts every peer) -- safe because BaseApp registers the filter
// (app/app.go's SetAddrPeerFilter) before node.NewNode() constructs the real
// Switch, but no real peer connection is ever attempted before n.Start()
// either, so there is no real window for a Sybil peer to exploit the
// fail-open gap between registration and wiring.
func (k *Keeper) SetPeerFilterSource(src PeerFilterSource) {
	k.peerFilterSrc = src
}

// FilterPeerByAddr is registered via baseapp.SetAddrPeerFilter (app/app.go).
// No CometBFT fork changes are needed for this hook to fire: node/setup.go's
// createCometTransport already builds an ABCI-query-based PeerFilterFunc
// (calling "/p2p/filter/addr/<addr>") whenever config.P2P.FilterPeers is
// true, and cosmos-sdk's baseapp/abci.go's handleQueryP2P already routes
// that path straight to whatever function was registered here.
//
// info is net.Conn.RemoteAddr().String() (e.g. "1.2.3.4:56789"). Rejects a
// new peer if admitting it would push its subnet's (types.SubnetOf) current
// connected-peer count to or above Params.MaxPeersPerSubnet -- the ACTIVE
// counterpart to the passive SubnetDiversity metric IsP2PQualityHealthy
// already reads (x/sovereignty/types/predicates.go): that metric only
// notices an eclipse/Sybil condition AFTER enough same-subnet peers have
// already connected and degraded the FSM; this stops the next one from
// connecting at all. Closes a real design-vs-implementation gap: an active
// ingress filter was designed in this project's own early P2P notes but
// never built -- only passive post-hoc detection existed until this.
//
// Fails open (accepts) on any parse failure or before SetPeerFilterSource is
// called -- this is a best-effort defense-in-depth layer, not the sole
// safety mechanism (MaxNumInboundPeers' existing CometBFT-level connection
// cap and the passive SubnetDiversity/IsP2PQualityHealthy degradation path
// both still apply regardless of this filter's outcome), matching this
// codebase's existing "best-effort health signal, not consensus-critical
// parsing" convention (cmd/engramd/main.go's parsePersistentPeerIDs).
func (k *Keeper) FilterPeerByAddr(info string) *abci.ResponseQuery {
	accept := &abci.ResponseQuery{}
	if k.peerFilterSrc == nil {
		return accept
	}
	host, _, err := net.SplitHostPort(info)
	if err != nil {
		host = info
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return accept
	}
	subnet := types.SubnetOf(ip)
	if k.peerFilterSrc.PeerCountInSubnet(subnet) >= k.Params.MaxPeersPerSubnet {
		return &abci.ResponseQuery{
			Code: 1,
			Log:  "rejected: MaxPeersPerSubnet reached for this peer's subnet",
		}
	}
	return accept
}
