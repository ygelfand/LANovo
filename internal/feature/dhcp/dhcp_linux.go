package dhcp

import (
	"context"
	"fmt"

	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
)

// There is no DHCP client on this device. AOSP dropped dhcpcd in O and does it inside
// system_server, which is only reachable through the wifi framework we are not using —
// /system/etc/dhcpcd-6.8.2/dhcpcd.conf is left over from before that change and the binary is gone.
//
// DHCP runs over AF_PACKET because a host with no address yet cannot use a UDP socket for it.
func dhcpRequest(ctx context.Context, iface string) (*Lease, error) {
	c, err := nclient4.New(iface)
	if err != nil {
		return nil, fmt.Errorf("dhcp: %w", err)
	}
	defer c.Close()

	got, err := c.Request(ctx, asking(previous())...)
	if err != nil {
		return nil, fmt.Errorf("dhcp: %w", err)
	}
	return leaseFrom(got.ACK)
}
