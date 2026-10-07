package dhcp

import (
	"log/slog"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"

	"github.com/ygelfand/LANovo/internal/config"
	shared "github.com/ygelfand/libcountertop/pkg/network/dhcp"
)

// previous is the address to ask for, or nil for a device that has not had one.
//
// RFC 2131 section 4.4.1: a client that remembers an address should name it in the requested-address
// option of its DISCOVER, and a server that can honor it will. Without it a reboot takes whatever is
// next in the pool, which moves the device out from under anything pointed at it by address — and
// with the release now sent on a clean stop, the pool is free to hand it straight to somebody else.
//
// Only IPv4, and only something that parses. What is in the file came from a lease, but the file is
// editable and a bad value here would be a malformed option on every request rather than a bad
// address once.
func previous() net.IP {
	said := config.Get().Network.Address
	if said == "" {
		return nil
	}

	addr := net.ParseIP(said)
	if addr == nil || addr.To4() == nil {
		slog.Warn("ignoring a remembered address that is not IPv4", "address", said)
		return nil
	}
	return addr.To4()
}

// asking is what goes into the exchange: the hostname, the options worth having, and the address to
// ask for when there is one.
//
// Split out so what the device asks for can be checked without a network. The exchange itself needs
// a packet socket and a server; which options go into it does not.
func asking(want net.IP) []dhcpv4.Modifier { return shared.RequestOptions(hostname(), want) }

// remember writes down the address that was granted, so the next start can ask for it.
//
// Only on a change. The lease is renewed every half life and the address is nearly always the same
// one, and rewriting the settings file for that would be a write every few hours for nothing.
func remember(addr net.IP) {
	said := ""
	if addr.To4() != nil {
		said = addr.To4().String()
	}

	if config.Get().Network.Address == said {
		return
	}

	if err := config.Set().Network().Address(said); err != nil {
		slog.Warn("could not remember the address", "address", said, "err", err)
	}
}
