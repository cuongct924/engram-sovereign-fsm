package types

import "net"

// SubnetOf masks ip to this codebase's peer-diversity granularity (IPv4
// /24, IPv6 /48). Shared by the passive SubnetDiversity metric and the
// active ingress filter (peer_filter.go's FilterPeerByAddr).
func SubnetOf(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.Mask(net.CIDRMask(24, 32)).String()
	}
	return ip.Mask(net.CIDRMask(48, 128)).String()
}
