// Package dhcp binds the shared lease client to product identity and persisted configuration.
package dhcp

import (
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/service"
	shared "github.com/ygelfand/libcountertop/pkg/network/dhcp"
	"github.com/ygelfand/libcountertop/pkg/runtime/startup"
)

const Interface = shared.Interface

type Client struct{ *shared.Client }

var (
	once         sync.Once
	sharedClient *Client
)

func init() {
	component.Register(
		component.Network,
		Get,
		component.Order(10),
		component.Supervise(service.Restart(2*time.Second, time.Minute)),
	)
}
func Get() *Client {
	once.Do(func() {
		sharedClient = &Client{
			Client: shared.New(
				shared.Options{
					Interface: Interface,
					Hostname:  hostname,
					Previous:  previous,
					Remember:  remember,
				},
			),
		}
	})
	return sharedClient
}
func (c *Client) Startup() component.Progress {
	var address string
	if l := c.Lease(); l != nil {
		address = l.Address.IP.String()
	}
	return startup.NetworkAddress(wifi.Get().Startup(), address)
}
