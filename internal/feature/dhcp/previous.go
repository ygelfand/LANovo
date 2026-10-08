package dhcp

import (
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4"
	shared "github.com/ygelfand/libcountertop/pkg/network/dhcp"

	"github.com/ygelfand/LANovo/internal/config"
)

func previous() net.IP                     { return shared.Previous(config.Get().Network.Address) }
func asking(want net.IP) []dhcpv4.Modifier { return shared.RequestOptions(hostname(), want) }
func remember(addr net.IP) {
	shared.Remember(
		addr,
		func() string { return config.Get().Network.Address },
		func(s string) error { return config.Set().Network().Address(s) },
	)
}
