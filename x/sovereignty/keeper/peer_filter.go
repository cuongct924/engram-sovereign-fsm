package keeper

import (
	"net"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cuongct220020/engram-sovereign-fsm/x/sovereignty/types"
)

// PeerFilterSource abstracts a live per-subnet connected-peer count --
// concretely cmd/engramd's vanillaP2PHealthAdapter, wired via
// SetPeerFilterSource. Avoids importing the CometBFT fork's p2p package
// directly (same pattern as sensors.P2PHealthSource).
type PeerFilterSource interface {
	PeerCountInSubnet(subnet string) uint64
}

// SetPeerFilterSource wires src in. Until called, FilterPeerByAddr fails
// open (accepts every peer) -- safe since no real peer connection happens
// before node.NewNode() constructs the Switch and wires this.
func (k *Keeper) SetPeerFilterSource(src PeerFilterSource) {
	k.peerFilterSrc = src
}

// FilterPeerByAddr is registered via baseapp.SetAddrPeerFilter (app/app.go)
// and called automatically by CometBFT's PeerFilterFunc when
// config.P2P.FilterPeers is true. info is net.Conn.RemoteAddr().String().
//
// Rejects a new peer if admitting it would push its subnet's
// (types.SubnetOf) connected-peer count to or above Params.MaxPeersPerSubnet
// -- the active counterpart to the passive SubnetDiversity metric, which
// only notices an eclipse/Sybil after enough same-subnet peers connected.
//
// Fails open on any parse failure or before SetPeerFilterSource is called --
// best-effort defense-in-depth, not the sole safety mechanism (CometBFT's
// MaxNumInboundPeers and the passive SubnetDiversity path still apply).
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
