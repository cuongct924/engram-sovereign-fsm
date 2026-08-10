package keeper

import (
	"net"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// PeerFilterSource abstracts a live per-subnet connected-peer count --
// concretely cmd/engramd's vanillaP2PHealthAdapter, wired via
// SetPeerFilterSource. Mirrors sensors.P2PHealthSource's pattern: this
// package doesn't import the CometBFT fork's p2p package directly.
type PeerFilterSource interface {
	PeerCountInSubnet(subnet string) uint64
}

// SetPeerFilterSource wires src in. Until called, FilterPeerByAddr fails
// open (accepts every peer) -- safe since no real peer connection happens
// before node.NewNode() constructs the Switch and this gets wired.
func (k *Keeper) SetPeerFilterSource(src PeerFilterSource) {
	k.peerFilterSrc = src
}

// FilterPeerByAddr is registered via baseapp.SetAddrPeerFilter (app/app.go);
// CometBFT's own ABCI-query-based PeerFilterFunc calls it automatically when
// config.P2P.FilterPeers is true.
//
// info is net.Conn.RemoteAddr().String(). Rejects a new peer if admitting it
// would push its subnet's (types.SubnetOf) connected-peer count to or above
// Params.MaxPeersPerSubnet -- the active counterpart to the passive
// SubnetDiversity metric, which only notices an eclipse/Sybil condition
// after enough same-subnet peers have already connected.
//
// Fails open on any parse failure or before SetPeerFilterSource is called --
// a best-effort defense-in-depth layer, not the sole safety mechanism
// (CometBFT's own MaxNumInboundPeers cap and the passive SubnetDiversity
// path both still apply regardless).
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
