package clock

import (
	"context"
	"log/slog"
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/hook"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"
	zonepolicy "github.com/ygelfand/libcountertop/pkg/settings/timezone"

	"github.com/ygelfand/LANovo/internal/component"
)

func init() {
	component.Register(sharedcomponent.Network, Get, sharedcomponent.Order(20),
		sharedcomponent.Supervise(service.Restart(5*time.Second, time.Minute)))
}

const (
	resync = time.Hour

	retry = 30 * time.Second

	settle = 10 * time.Second

	step = time.Second
)

type Clock struct {
	Synced hook.Hook[time.Time]

	Stepped hook.Hook[time.Duration]

	mu     sync.Mutex
	offset time.Duration
	at     time.Time

	ready chan struct{}
	once  sync.Once

	asked *esphome.Conn

	zone       *esphome.Select
	zonePolicy *zonepolicy.Policy
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

func (c *Clock) Ready() <-chan struct{} { return c.ready }

func (c *Clock) Offset() (time.Duration, time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.offset, c.at
}

func (c *Clock) Set() bool {
	select {
	case <-c.ready:
		return true
	default:
		return false
	}
}

func (c *Clock) Run(ctx context.Context) error {
	if err := sleep(ctx, settle); err != nil {
		return err
	}

	for {
		// The PMIC's RTC counts from power-on and refuses RTC_SET_TIME; nothing disciplines the kernel clock.
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

func (c *Clock) WatchSteps(changed func(time.Duration)) func() {
	return c.Stepped.Listen(changed)
}
