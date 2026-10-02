package network

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/metrics"
)

const retry = 3 * time.Second

// Advertise keeps something published on the device's addresses, publishing again whenever they change, until ctx ends.
func Advertise(ctx context.Context, what string, publish func(ips []net.IP) (stop func(), err error)) {
	for attempt := 1; ; attempt++ {
		ips := metrics.Settled(ctx, retry)
		if len(ips) == 0 {
			if attempt == 1 {
				slog.Info("waiting for an address before advertising", "what", what)
			}
			if !pause(ctx) {
				return
			}
			continue
		}

		stop, err := publish(ips)
		if err != nil {
			if attempt == 1 {
				slog.Warn("advertising failed, retrying", "what", what, "err", err)
			}
			if !pause(ctx) {
				return
			}
			continue
		}

		key := metrics.AddressKey(ips)
		slog.Info("advertising", "what", what, "addrs", key)

		for now := key; now == key || now == ""; now = metrics.AddressKey(metrics.Addresses()) {
			if !pause(ctx) {
				stop()
				return
			}
		}
		stop()
		slog.Info("addresses changed, advertising again", "what", what, "was", key)
	}
}

// Strings is the addresses the way zeroconf takes them.
func Strings(ips []net.IP) []string {
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ip.String())
	}
	return out
}

func pause(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(retry):
		return true
	}
}
