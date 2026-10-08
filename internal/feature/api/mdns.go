package api

import (
	"context"
	"net"

	"github.com/ygelfand/go-esphome-device/mdns"
	netadvert "github.com/ygelfand/libcountertop/pkg/network/advertise"

	"github.com/ygelfand/LANovo/internal/layout"
)

// The mdns library advertises loopback when given no addresses.
func (a *API) advertise(ctx context.Context, port int) {
	netadvert.Advertise(ctx, "esphome", func(ips []net.IP) (func(), error) {
		adv, err := mdns.Advertise(mdns.Config{
			Name:          a.name,
			FriendlyName:  a.srv.Info.FriendlyName,
			Port:          port,
			MACAddress:    a.srv.Info.MACAddress,
			Version:       a.srv.Info.Version,
			Platform:      layout.Platform,
			Board:         layout.Board,
			Encrypted:     a.srv.PSK != nil && !a.srv.PSK.IsZero(),
			Provisionable: a.srv.PSK != nil && a.srv.PSK.IsZero(),
			IPs:           ips,
		})
		if err != nil {
			return nil, err
		}
		return func() { _ = adv.Close() }, nil
	})
}
