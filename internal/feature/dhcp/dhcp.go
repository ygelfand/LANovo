// Package dhcp keeps an address on an interface.
//
// There is no DHCP client on this device: AOSP dropped dhcpcd in O and does it inside
// system_server. So lanovod holds the lease itself — asks for one when the link comes up, renews
// before it expires, and asks again when the link goes away and returns.
//
// It watches the interface rather than being told about it, so it has nothing to do with what
// brought the link up.
package dhcp

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/service"
	"github.com/ygelfand/libcountertop/pkg/hook"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
)

func init() {
	// Before anything that reaches outside, which is what the phase is for.
	component.Register(component.Network, Get, component.Order(10),
		component.Supervise(service.Restart(2*time.Second, time.Minute)))
}

// Interface is what the lease is held on.
const Interface = "wlan0"

// Client holds one lease.
type Client struct {
	Leased hook.Hook[*Lease]

	mu    sync.Mutex
	lease *Lease
}

var (
	once   sync.Once
	shared *Client
)

func Get() *Client { once.Do(func() { shared = &Client{} }); return shared }

func (c *Client) Name() string { return "dhcp" }

// Startup is not ready until the device has an address.
func (c *Client) Startup() component.Progress {
	lease := c.Lease()
	if lease == nil {
		return component.Progress{Doing: "asking for an address"}
	}
	return component.Progress{Done: true, Doing: lease.Address.IP.String()}
}

func (c *Client) Wait(ctx context.Context) bool {
	got := make(chan struct{}, 1)
	stop := c.Leased.Listen(func(l *Lease) {
		if l == nil {
			return
		}
		select {
		case got <- struct{}{}:
		default:
		}
	})
	defer stop()
	if c.Lease() != nil {
		return true
	}
	select {
	case <-got:
		return true
	case <-ctx.Done():
		return false
	}
}

// Lease is the address currently held, or nil when there is none.
func (c *Client) Lease() *Lease {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lease
}

// Run holds an address for as long as the link is up.
func (c *Client) Run(ctx context.Context) error {
	for {
		if err := waitLink(ctx); err != nil {
			return err
		}

		lease, err := c.obtain(ctx)
		if err != nil {
			// A link that is up but not carrying is the usual reason, and it is worth another go
			// rather than a restart.
			slog.Warn("no lease", "err", err)
			if err := sleep(ctx, 5*time.Second); err != nil {
				return err
			}
			continue
		}

		if taken, err := c.contested(ctx, lease); err != nil {
			return err
		} else if taken {
			continue
		}

		if err := apply(Interface, lease); err != nil {
			return err
		}
		useDNS(lease.DNS)
		// c still holds the lease this one replaces: set comes after.
		if err := useNetd(Interface, c.Lease(), lease); err != nil {
			slog.Warn("netd would not take the lease", "err", err)
		}

		// Said out loud, on its own goroutine: three announcements two seconds apart is four
		// seconds nothing else should wait for.
		safe.Go("arp announce", func() {
			if err := announce(Interface, lease.Address.IP); err != nil {
				slog.Warn("could not announce the address", "err", err)
			}
		})
		c.set(lease)
		remember(lease.Address.IP)
		slog.Info("address", "lease", lease.String())

		// Answering for the address is part of holding it. The watch lives exactly as long as this
		// lease does, so a new one is defended by a watch that knows the address it is defending.
		watch, stop := context.WithCancel(ctx)
		taken := make(chan struct{}, 1)
		safe.Go("arp defend", func() {
			err := watching(watch, Interface, lease.Address.IP, func() {
				select {
				case taken <- struct{}{}:
				default:
				}
			})
			if err != nil {
				slog.Warn("not watching for address conflicts", "err", err)
			}
		})

		err = c.hold(ctx, lease, taken)
		stop()

		if err != nil {
			return err
		}
	}
}

// obtain keeps the address the device already has where it can, and asks for one where it cannot.
//
// A renewal is not a fresh negotiation. RFC 2131 section 4.4.5 has the client ask the server that
// granted the lease to extend it, and only fall back to the full exchange once that has run out of
// time — which is what keeps the address, and everything bound to it, across a renewal.
func (c *Client) obtain(ctx context.Context) (*Lease, error) {
	held := c.Lease()
	if held == nil {
		return dhcpRequest(ctx, Interface)
	}

	got, err := extend(ctx, Interface, held)
	switch {
	case err == nil:
		return got, nil
	case ctx.Err() != nil:
		return nil, err
	}

	// Either the server refused or nobody answered before the lease ran out. The address is not
	// the device's any more, so it goes back to asking from nothing.
	slog.Warn("could not renew the address", "address", held.Address.IP, "err", err)
	c.set(nil)
	release(Interface)

	return dhcpRequest(ctx, Interface)
}

// Close gives the lease back and drops the address, so a restart does not leave a stale one behind.
//
// The release goes first, while the address is still on the interface: it is sent from that address
// and the kernel has to be able to route it.
func (c *Client) Close() error {
	if lease := c.Lease(); lease != nil {
		if err := releasing(Interface, lease); err != nil {
			slog.Warn("could not release the address", "err", err)
		}
	}

	c.set(nil)
	release(Interface)
	return nil
}

// Release is what a restart does instead of closing, and it is deliberately nothing.
//
// Close gives the lease back and drops the address, which is right when the device is stopping and
// wrong between two runs of this loop: the address would go, every connection with it, and the
// server would be free to hand it to somebody else before the restart had asked for it again. A
// restart that then failed would leave the device on the network with no address at all.
//
// Keeping it also puts the restart on the better path. Run picks up with the lease still held, so
// obtain renews it rather than negotiating from nothing, which is what RFC 2131 wants from a
// client that still has an address.
func (c *Client) Release() error { return nil }

func (c *Client) set(l *Lease) {
	c.mu.Lock()
	was := c.lease
	c.lease = l
	c.mu.Unlock()
	if l != was {
		c.Leased.Emit(l)
	}
}

// backoff is how long to leave a contested address alone. RFC 5227 section 2.1 asks for ten
// seconds before trying again, so a pair of hosts fighting over one address does not fight fast.
const backoff = 10 * time.Second

// probing and declining are the two halves that need a packet socket, behind variables so a test
// can see whether each was reached. Whether they are called at all is the part worth pinning: one
// takes seconds of listening, and the other is the difference between a conflict being reported
// and being looped on.
var (
	probing   = probe
	declining = sendDecline

	// releasing is here too, so a test can see that stopping gives the lease back rather than
	// letting the server hold it until it expires.
	releasing = sendRelease
)

// contested reports whether something else on the segment already holds the offered address, and
// waits before the next attempt when it does.
//
// Only for an address the device is not already using. RFC 2131 section 2.2 wants a new address
// checked before it is taken; an address being renewed has been in use all along, and probing for
// it would mean listening for an answer this device is the one giving.
func (c *Client) contested(ctx context.Context, lease *Lease) (bool, error) {
	if held := c.Lease(); held != nil && held.Address.IP.Equal(lease.Address.IP) {
		return false, nil
	}

	taken, err := probing(Interface, lease.Address.IP)
	if err != nil {
		// The probe itself failing is not evidence. Taking the address is what a client that
		// cannot probe at all does, and it is what this one did until now.
		slog.Warn("could not check whether the address is free", "address", lease.Address.IP, "err", err)
		return false, nil
	}
	if !taken {
		return false, nil
	}

	slog.Warn("refusing an address something else is using", "address", lease.Address.IP, "server", lease.Server)

	// Told, not just refused. Without this the server offers the same address straight back and a
	// real conflict becomes a loop on the backoff.
	if err := declining(Interface, lease.Address.IP, lease.Server, "address already in use"); err != nil {
		slog.Warn("could not tell the server", "err", err)
	}
	return true, sleep(ctx, backoff)
}

// hold waits until the lease wants renewing, the address is lost, or the link has gone away.
func (c *Client) hold(ctx context.Context, lease *Lease, taken <-chan struct{}) error {
	renew := time.NewTimer(time.Until(lease.Renew))
	defer renew.Stop()

	check := time.NewTicker(15 * time.Second)
	defer check.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-renew.C:
			return nil
		case <-taken:
			// Surrendered to another host. The address is dropped rather than renewed: asking to
			// extend one this device has stopped using would get it back.
			if err := declining(Interface, lease.Address.IP, lease.Server, "address in use elsewhere"); err != nil {
				slog.Warn("could not tell the server", "err", err)
			}
			c.set(nil)
			release(Interface)
			return sleep(ctx, backoff)
		case <-check.C:
			if !linkUp() {
				slog.Warn("link went away, asking again")
				c.set(nil)
				return nil
			}
		}
	}
}

// waitLink blocks until the interface is carrying.
func waitLink(ctx context.Context) error {
	for {
		if linkUp() {
			return nil
		}
		if err := sleep(ctx, time.Second); err != nil {
			return err
		}
	}
}

// linkUp reports whether the interface is up and carrying. operstate is the kernel's own answer,
// so it covers any interface without knowing what drives it.
func linkUp() bool {
	b, err := os.ReadFile("/sys/class/net/" + Interface + "/operstate")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(b)) == "up"
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
