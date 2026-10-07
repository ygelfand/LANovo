package dhcp

import (
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/ygelfand/LANovo/internal/config"
	shared "github.com/ygelfand/libcountertop/pkg/network/dhcp"
	"net"
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
