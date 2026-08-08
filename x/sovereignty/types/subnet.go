package types

import "net"

// SubnetOf masks ip to the granularity used throughout this codebase for
// peer-diversity/anti-Sybil computations: IPv4 masked to /24, IPv6 to /48.
// Originally inline in cmd/engramd/main.go's vanillaP2PHealthAdapter.
// PeerHealthSnapshot (the passive SubnetDiversity metric); factored out here
// so the ingress peer filter (keeper/peer_filter.go's FilterPeerByAddr, the
// active counterpart) computes subnet membership identically -- the two must
// never drift, or a peer the filter admits and the sensor's diversity count
// disagree about which subnet it belongs to.
func SubnetOf(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.Mask(net.CIDRMask(24, 32)).String()
	}
	return ip.Mask(net.CIDRMask(48, 128)).String()
}
