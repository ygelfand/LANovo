package dhcp

import (
	"fmt"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/layout"
)

// keeping reports whether an address already on the interface is the one a lease is for, and so
// must be left alone rather than flushed.
//
// A renewal usually comes back with the address the device already had. Deleting it takes every
// socket bound to it with it, so the connection to Home Assistant, the web server and mDNS would
// all drop each time the lease came round.
func keeping(addr net.IP, prefix uint8, want net.IP, wantPrefix uint8) bool {
	if want == nil || addr == nil {
		return false
	}
	return prefix == wantPrefix && addr.Equal(want)
}

// Lease is what the DHCP server gave us.
type Lease struct {
	Address net.IPNet
	Router  net.IP
	DNS     []net.IP
	NTP     []net.IP
	Domain  string
	Server  net.IP

	// Expires is when the lease runs out. Renew is T1, when to start asking the server that granted
	// it; Rebind is T2, when to stop waiting for that one and ask the segment. Halfway and seven
	// eighths of the way through unless the server said otherwise.
	Expires time.Time
	Renew   time.Time
	Rebind  time.Time
}

// leaseFrom reads an ACK. Shared between the first one and every renewal, so what a lease means
// cannot drift between the two paths.
func leaseFrom(ack *dhcpv4.DHCPv4) (*Lease, error) {
	mask := ack.SubnetMask()
	if mask == nil {
		// A server that did not say assumes classful, which is not something to guess at.
		return nil, fmt.Errorf("dhcp: the lease carries no subnet mask")
	}

	lease := &Lease{
		Address: net.IPNet{IP: ack.YourIPAddr, Mask: mask},
		DNS:     ack.DNS(),
		NTP:     ack.NTPServers(),
		Domain:  ack.DomainName(),
		Server:  ack.ServerIdentifier(),
	}
	if routers := ack.Router(); len(routers) > 0 {
		lease.Router = routers[0]
	}

	now := time.Now()
	life := ack.IPAddressLeaseTime(time.Hour)
	lease.Expires = now.Add(life)

	lease.Renew = now.Add(life / 2)
	if t1 := ack.IPAddressRenewalTime(0); t1 > 0 {
		lease.Renew = now.Add(t1)
	}

	lease.Rebind = now.Add(life * 7 / 8)
	if t2 := ack.IPAddressRebindingTime(0); t2 > 0 {
		lease.Rebind = now.Add(t2)
	}

	// A server is allowed to say anything, and some say T2 before T1. Kept in order, because the
	// renewal walks them in sequence and a T2 in the past would skip renewing altogether.

	if !lease.Rebind.After(lease.Renew) {
		lease.Rebind = lease.Renew.Add(life / 8)
	}
	if !lease.Expires.After(lease.Rebind) {
		lease.Expires = lease.Rebind
	}
	return lease, nil
}

func (l Lease) String() string {
	return fmt.Sprintf("%s via %s, dns %s, for %s",
		l.Address.String(), l.Router, joinIPs(l.DNS), time.Until(l.Expires).Round(time.Second))
}

// hostname is what the device asks the server to call it.
//
// The name someone gave it, slugged the way the ESPHome node name is, so the device answers to one
// name on the network however it is looked up. Not the kernel's hostname: this board's is
// localhost and nothing sets it.
func hostname() string {
	if slug := layout.Slug(config.Get().Device.Name); slug != "" {
		return slug
	}
	return Fallback
}

// Fallback is the hostname a device with no name of its own asks for.
const Fallback = "lanovo"

func joinIPs(ips []net.IP) string {
	if len(ips) == 0 {
		return "(none)"
	}
	s := ips[0].String()
	for _, ip := range ips[1:] {
		s += ", " + ip.String()
	}
	return s
}
