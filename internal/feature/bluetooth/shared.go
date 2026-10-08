package bluetooth

import (
	"sync"

	sharedproxy "github.com/ygelfand/libcountertop/pkg/bluetooth/proxy"
	sharedwifi "github.com/ygelfand/libcountertop/pkg/network/wifi"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/ble"
	"github.com/ygelfand/LANovo/internal/layout"
)

type Proxy struct{ *sharedproxy.Proxy }

func init() { component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(40)) }

var get = sync.OnceValue(func() *Proxy {
	return &Proxy{sharedproxy.New(sharedproxy.Dependencies{
		Radio:     ble.Get(),
		Settings:  config.BluetoothSection,
		Reconnect: func() { component.Reconnect.Emit(struct{}{}) },
		Beacon: func() []byte {
			mac, _ := sharedwifi.ReadMAC(layout.WifiIface)
			return sharedproxy.BeaconAdvertisement(sharedproxy.BeaconMinor(mac))
		},
	})}
})

func Get() *Proxy                      { return get() }
func (p *Proxy) Restore(config.Config) { p.Proxy.Restore() }
