package sendspin

import (
	"context"
	core "github.com/ygelfand/libcountertop/pkg/audio/sendspin"
	"net"

	"github.com/libp2p/zeroconf/v2"

	"github.com/ygelfand/LANovo/internal/feature/network"
)

// What a server browses for. The path is required by the spec — it is how a server knows where to
// point its WebSocket once mDNS has told it the address.
const (
	service = core.Service
	domain  = core.Domain
	path    = core.Path
)

// advertise publishes the room under this device's own host name; zeroconf falls back to loopback without addresses.
func advertise(ctx context.Context, name string, port int) {
	network.Advertise(ctx, "sendspin", func(ips []net.IP) (func(), error) {
		srv, err := zeroconf.RegisterProxy(
			name, service, domain, port, name, network.Strings(ips),
			[]string{"path=" + path, "name=" + name},
			nil,
		)
		if err != nil {
			return nil, err
		}
		return srv.Shutdown, nil
	})
}
