package bluetooth

import (
	"sync"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/ble"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	sharedproxy "github.com/ygelfand/libcountertop/pkg/bluetooth/proxy"
)

type Proxy struct{ *sharedproxy.Proxy }

const Features = sharedproxy.Features

func init() { component.Register(component.Device, Get, component.Order(40)) }

var get = sync.OnceValue(func() *Proxy {
	return &Proxy{sharedproxy.New(sharedproxy.Dependencies{
		Radio:     ble.Get(),
		Settings:  config.BluetoothSection,
		Reconnect: func() { component.Reconnect.Emit(struct{}{}) },
		Beacon: func() []byte {
			mac, _ := wifi.MAC()
			return sharedproxy.BeaconAdvertisement(sharedproxy.BeaconMinor(mac))
		},
	})}
})

func Get() *Proxy                      { return get() }
func (p *Proxy) Restore(config.Config) { p.Proxy.Restore() }
