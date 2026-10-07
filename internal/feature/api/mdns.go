package api

import (
	"context"
	"net"

	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/go-esphome-device/mdns"
	netadvert "github.com/ygelfand/libcountertop/pkg/network/advertise"
)

// advertise publishes the addresses the device actually has; the library falls back to loopback without them.
func (a *API) advertise(ctx context.Context, port int) {
	netadvert.Advertise(ctx, "esphome", func(ips []net.IP) (func(), error) {
		adv, err := mdns.Advertise(mdns.Config{
			Name:         a.name,
			FriendlyName: a.srv.Info.FriendlyName,
			Port:         port,
			MACAddress:   a.srv.Info.MACAddress,
			Version:      a.srv.Info.Version,
			Platform:     layout.Platform,
			Board:        layout.Board,
			// A zero key is not encryption to advertise: it says the device is waiting to be
			// given one, which is what stops Home Assistant asking for a key nobody has.
			Encrypted:     a.srv.PSK != nil && !a.srv.PSK.IsZero(),
			Provisionable: a.srv.PSK != nil && a.srv.PSK.IsZero(),
			IPs:           ips,
		})
		if err != nil {
			return nil, err
		}
		return func() { adv.Close() }, nil
	})
}
