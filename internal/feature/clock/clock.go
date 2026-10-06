// Package clock is what time it is, and where.
//
// The PMIC's RTC counts from power-on and refuses RTC_SET_TIME, and the kernel sets the system
// clock from it at boot, so every boot starts in 1970. Nothing disciplines the kernel's clock
// afterwards either, so it is checked again on a timer for as long as the device is up.
//
// Home Assistant is the first source asked and answers with the time zone as well. Time servers
// are the fallback. Anything validating a certificate waits on Ready.
package clock

import (
	"context"
	"log/slog"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/service"
	"github.com/ygelfand/libcountertop/pkg/hook"
)

func init() {
	// After dhcp: the servers come from the lease, and reaching any of them needs an address.
	component.Register(component.Network, Get, component.Order(20),
		component.Supervise(service.Restart(5*time.Second, time.Minute)))
}

const (
	// resync is how often the clock is checked once it is right.
	resync = time.Hour

	// retry is the wait after a round that reached nobody.
	retry = 30 * time.Second

	// settle is how long Home Assistant is given to answer before a time server is asked. It is
	// the better source and usually the faster one, since it is already connected.
	settle = 10 * time.Second

	// step is how far out the clock has to be before it is worth setting.
	step = time.Second
)

// Clock is the device's time.
type Clock struct {
	// Synced carries the time each sync settled on.
	Synced hook.Hook[time.Time]

	Stepped hook.Hook[time.Duration]

	mu     sync.Mutex
	offset time.Duration
	at     time.Time

	// ready is closed by the first sync and never reopened, so a waiter that arrives afterwards
	// still returns at once.
	ready chan struct{}
	once  sync.Once

	// asked is the connection Home Assistant was last asked on. Per connection rather than once: a
	// client that reconnects is a client that can be asked again, and after a long disconnection it
	// is the better source.
	asked *esphome.Conn

	// zone is the override: which place the device keeps time by, whatever the server says.
	zone *esphome.Select
}

var (
	once   sync.Once
	shared *Clock
)

func Get() *Clock {
	once.Do(func() {
		shared = &Clock{ready: make(chan struct{})}
		shared.buildZone()
	})
	return shared
}

func (c *Clock) Name() string { return "clock" }

// Ready is closed once the clock has been set. Anything that validates a certificate waits on it.
func (c *Clock) Ready() <-chan struct{} { return c.ready }

// Offset is how far out the clock was at the last sync, and when that was.
func (c *Clock) Offset() (time.Duration, time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.offset, c.at
}

// Set reports whether the clock has been set by anything.
func (c *Clock) Set() bool {
	select {
	case <-c.ready:
		return true
	default:
		return false
	}
}

// Run keeps asking until it has the time, then checks it on a timer.
//
// Home Assistant may have answered first, over the API. That is the better source on a network
// with no route out, so the first round waits a moment for it before reaching for a server.
func (c *Clock) Run(ctx context.Context) error {
	if err := sleep(ctx, settle); err != nil {
		return err
	}

	for {
		// Every round, not only until the clock is first right. Nothing on this board keeps time
		// across a boot and nothing disciplines the kernel's clock, so a device left running drifts
		// for as long as it is up. accept only steps the clock when it is worth stepping, so a
		// round that finds nothing wrong costs one exchange.
		wait := resync
		if err := c.sync(ctx); err != nil {
			slog.Warn("no time yet", "err", err)
			wait = retry
		}

		if err := sleep(ctx, wait); err != nil {
			return err
		}
	}
}

// accept takes the offset one source reported, and sets the clock if it is worth setting.
func (c *Clock) accept(offset time.Duration, source string) error {
	if worthSetting(offset) {
		now := time.Now().Add(offset)
		if err := setClock(now); err != nil {
			return err
		}
		slog.Info("clock set", "source", source, "by", offset.Round(time.Millisecond), "now", now)
		defer c.Stepped.Emit(offset)
	}

	c.mu.Lock()
	c.offset, c.at = offset, time.Now()
	c.mu.Unlock()

	c.once.Do(func() { close(c.ready) })
	c.Synced.Emit(time.Now())
	return nil
}

// worthSetting keeps the clock still for the drift a stepped clock accumulates between syncs.
func worthSetting(offset time.Duration) bool { return offset > step || offset < -step }

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
