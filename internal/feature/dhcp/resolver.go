package dhcp

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"
)

// timeout bounds reaching one nameserver, so one that is not answering falls through to the next
// rather than holding up whatever asked.
const timeout = 5 * time.Second

var (
	mu      sync.Mutex
	servers []string
	install sync.Once
)

// useDNS makes every lookup in this process go to the nameservers this lease named, by replacing
// the resolver the standard library shares. Nothing else has to be told: http.DefaultClient
// included.
//
// There is no /etc/resolv.conf on this device, so Go's resolver has nowhere to look. bionic asks
// netd instead, which nothing configures once system_server is gone.
func useDNS(ips []net.IP) {
	addrs := make([]string, 0, len(ips))
	for _, ip := range ips {
		addrs = append(addrs, ip.String())
	}

	mu.Lock()
	servers = addrs
	mu.Unlock()

	install.Do(func() { net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: dial} })
	slog.Info("resolver configured", "nameservers", joinIPs(ips))
}

// dial ignores the address Go derived from the resolv.conf it could not find and uses the lease's,
// trying each in turn.
func dial(ctx context.Context, network, _ string) (net.Conn, error) {
	mu.Lock()
	addrs := servers
	mu.Unlock()

	if len(addrs) == 0 {
		return nil, errors.New("dhcp: the lease named no nameservers")
	}

	var err error
	for _, server := range addrs {
		d := net.Dialer{Timeout: timeout}

		var conn net.Conn
		if conn, err = d.DialContext(ctx, network, net.JoinHostPort(server, "53")); err == nil {
			return conn, nil
		}
	}
	return nil, err
}
