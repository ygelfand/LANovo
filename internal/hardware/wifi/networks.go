package wifi

import (
	"context"
	"github.com/ygelfand/libcountertop/pkg/network/wpa"
)

type Network = wpa.Network
type Security = wpa.Security

var networks = wpa.NetworksClient{Dial: func() (wpa.Connection, error) { return Dial() }}

func Scan(ctx context.Context) ([]Network, error) { return networks.Scan(ctx) }
func Networks() ([]string, error)                 { return networks.Networks() }
func Join(ctx context.Context, ssid, passphrase string) error {
	return networks.Join(ctx, ssid, passphrase)
}
func Forget(ssid string) error { return networks.Forget(ssid) }
