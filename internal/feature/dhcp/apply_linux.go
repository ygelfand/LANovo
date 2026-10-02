package dhcp

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/jsimonetti/rtnetlink"
	"golang.org/x/sys/unix"
)

// apply puts the lease on the interface: the address, then the default route.
//
// Over rtnetlink rather than ip(8). The binary is a toybox applet on a system LANovo is actively
// stripping, and a service that stops working because an applet went away is a bad way to lose the
// network.
func apply(iface string, l *Lease) error {
	link, err := net.InterfaceByName(iface)
	if err != nil {
		return fmt.Errorf("dhcp: %s: %w", iface, err)
	}

	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("dhcp: rtnetlink: %w", err)
	}
	defer conn.Close()

	ones, _ := l.Address.Mask.Size()
	addr := l.Address.IP.To4()
	if addr == nil {
		return fmt.Errorf("dhcp: %s is not an IPv4 address", l.Address.IP)
	}

	held, err := flush(conn, link.Index, addr, uint8(ones))
	if err != nil {
		return err
	}

	if !held {
		// The broadcast address the kernel would otherwise have to be told separately.
		brd := make(net.IP, net.IPv4len)
		binary.BigEndian.PutUint32(brd,
			binary.BigEndian.Uint32(addr)|^binary.BigEndian.Uint32(net.IP(l.Address.Mask).To4()))

		if err := conn.Address.New(&rtnetlink.AddressMessage{
			Family:       unix.AF_INET,
			PrefixLength: uint8(ones),
			Scope:        unix.RT_SCOPE_UNIVERSE,
			Index:        uint32(link.Index),
			Attributes: &rtnetlink.AddressAttributes{
				Address:   addr,
				Local:     addr,
				Broadcast: brd,
			},
		}); err != nil {
			return fmt.Errorf("dhcp: adding %s: %w", l.Address.String(), err)
		}
	}

	if l.Router == nil {
		return nil
	}

	// Replace rather than add: a default route left by a previous lease is not an error to hit.
	if err := conn.Route.Replace(&rtnetlink.RouteMessage{
		Family:    unix.AF_INET,
		Table:     unix.RT_TABLE_MAIN,
		Protocol:  unix.RTPROT_DHCP,
		Scope:     unix.RT_SCOPE_UNIVERSE,
		Type:      unix.RTN_UNICAST,
		DstLength: 0,
		Attributes: rtnetlink.RouteAttributes{
			Dst:      net.IPv4zero,
			Gateway:  l.Router.To4(),
			OutIface: uint32(link.Index),
		},
	}); err != nil {
		return fmt.Errorf("dhcp: default route via %s: %w", l.Router, err)
	}
	return nil
}

// flush removes the IPv4 addresses on the interface apart from the one the lease is for, and says
// whether that one was already there.
//
// The exception is the point. Deleting an address takes every socket bound to it with it, and a
// lease is usually renewed onto the address it already had — so flushing first would drop the
// connection to Home Assistant, the web server and mDNS every time the lease came up, which on
// this network is every half hour.
func flush(conn *rtnetlink.Conn, index int, keep net.IP, prefix uint8) (held bool, err error) {
	addrs, err := conn.Address.List()
	if err != nil {
		return false, fmt.Errorf("dhcp: listing addresses: %w", err)
	}

	for _, a := range addrs {
		if a.Index != uint32(index) || a.Family != unix.AF_INET {
			continue
		}
		if a.Attributes != nil && keeping(a.Attributes.Address, a.PrefixLength, keep, prefix) {
			held = true
			continue
		}
		if err := conn.Address.Delete(&a); err != nil {
			return held, fmt.Errorf("dhcp: removing an address: %w", err)
		}
	}
	return held, nil
}

// release drops whatever the lease put on, for a clean stop.
func release(iface string) {
	link, err := net.InterfaceByName(iface)
	if err != nil {
		return
	}
	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// Nothing kept: this is the device letting the address go, not renewing it.
	flush(conn, link.Index, nil, 0)
}
