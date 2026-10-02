package clock

import (
	"context"
	"fmt"
	"time"

	"github.com/beevik/ntp"

	"github.com/ygelfand/LANovo/internal/feature/dhcp"
)

// The fallback source: whatever the lease named, or the pool. Reaching any of them needs a route
// out, which Home Assistant does not.

// timeout bounds one exchange.
const timeout = 5 * time.Second

// pool is where the time comes from when the lease names no server.
var pool = []string{"0.pool.ntp.org", "1.pool.ntp.org", "2.pool.ntp.org"}

// sync asks each server in turn and stops at the first that answers.
func (c *Clock) sync(ctx context.Context) error {
	servers := c.servers()
	if len(servers) == 0 {
		return fmt.Errorf("clock: no servers")
	}

	var err error
	for _, server := range servers {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var r *ntp.Response
		if r, err = ntp.QueryWithOptions(server, ntp.QueryOptions{Timeout: timeout}); err != nil {
			continue
		}
		if err = r.Validate(); err != nil {
			continue
		}
		return c.accept(r.ClockOffset, server)
	}
	return err
}

// servers is what the lease named, or the pool when it named nobody.
func (c *Clock) servers() []string {
	lease := dhcp.Get().Lease()
	if lease == nil || len(lease.NTP) == 0 {
		return pool
	}

	servers := make([]string, 0, len(lease.NTP))
	for _, ip := range lease.NTP {
		servers = append(servers, ip.String())
	}
	return servers
}
