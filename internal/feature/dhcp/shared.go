package dhcp

import (
	"sync"
	"time"

	shared "github.com/ygelfand/libcountertop/pkg/network/dhcp"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"
	"github.com/ygelfand/libcountertop/pkg/runtime/startup"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
)

type Client struct{ *shared.Client }

var (
	once         sync.Once
	sharedClient *Client
)

func init() {
	component.Register(
		sharedcomponent.Network,
		Get,
		sharedcomponent.Order(10),
		sharedcomponent.Supervise(service.Restart(2*time.Second, time.Minute)),
	)
}
func Get() *Client {
	once.Do(func() {
		sharedClient = &Client{
			Client: shared.New(shared.Options{Hostname: hostname, Store: saved{}}),
		}
	})
	return sharedClient
}
func (c *Client) Startup() sharedcomponent.Progress {
	var address string
	if l := c.Lease(); l != nil {
		address = l.Address.IP.String()
	}
	return startup.NetworkAddress(wifi.Get().Startup(), address)
}
