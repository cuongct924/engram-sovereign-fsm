package types

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSubnetOfIPv4(t *testing.T) {
	require.Equal(t, "192.168.1.0", SubnetOf(net.ParseIP("192.168.1.203")))
	require.Equal(t, "10.20.30.0", SubnetOf(net.ParseIP("10.20.30.99")))
}

func TestSubnetOfIPv6(t *testing.T) {
	require.Equal(t, "2001:db8:1234::", SubnetOf(net.ParseIP("2001:db8:1234:5678::1")))
}

func TestSubnetOfIPv4MappedIPv6(t *testing.T) {
	require.Equal(t, "10.0.0.0", SubnetOf(net.ParseIP("::ffff:10.0.0.7")))
}
