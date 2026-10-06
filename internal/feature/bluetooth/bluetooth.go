// Package bluetooth forwards what the radio hears to Home Assistant, so a device out of range of
// everything else is in range of this one.
package bluetooth

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	esphome "github.com/ygelfand/go-esphome-device"
	"github.com/ygelfand/go-esphome-device/api"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/ble"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
)

func init() {
	component.Register(component.Device, Get, component.Order(40))
}

const (
	// batch is how many reports go in one message, and hold how long to wait for them.
	batch = 16
	hold  = 250 * time.Millisecond

	// queued is how many reports may wait for Home Assistant. Past this they are dropped: the radio
	// must never wait on the network, or the controller's own queue overflows and the driver starts
	// discarding packets for everything on the chip, wifi included.
	queued = 512
)

// Features is what the device advertises when the proxy is on. Active is how the scan is run
// rather than another declaration, and connections are absent because this proxy never makes one.
const Features = esphome.BluetoothPassiveScan |
	esphome.BluetoothRawAdvertisements |
	esphome.BluetoothStateAndMode

// Proxy carries what the radio hears to Home Assistant. The switch says whether it may scan, the
// subscription whether anyone is reading.
//
// Delivery and tuning are their own goroutines: handlers run on the connection's, and turning a
// scan on waits for the controller.
type Proxy struct {
	proxy  *esphome.BluetoothProxy
	radio  *ble.Radio
	enable *esphome.Switch

	// wanted nudges the tuner. One slot: it reads current state, so several requests are one.
	wanted  chan struct{}
	reports chan ble.Advertisement
	dropped atomic.Uint64

	mu      sync.Mutex
	stop    context.CancelFunc
	running bool
	active  bool
}

var (
	once   sync.Once
	shared *Proxy
)

func Get() *Proxy {
	once.Do(func() { shared = build() })
	return shared
}

func build() *Proxy {
	b := &Proxy{
		proxy:   &esphome.BluetoothProxy{},
		radio:   ble.Get(),
		wanted:  make(chan struct{}, 1),
		reports: make(chan ble.Advertisement, queued),
		enable: &esphome.Switch{
			Base: esphome.Base{
				ObjectID: "bluetooth_proxy",
				Name:     "Bluetooth proxy",
				Icon:     "mdi:bluetooth",
				Category: esphome.CategoryConfig,
			},
		},
	}

	b.proxy.OnSubscribed = func(bool) { b.apply() }
	b.proxy.OnMode = func(bool) { b.apply() }

	b.enable.Set(config.Get().Bluetooth.Proxy)
	b.enable.OnCommand = b.SetProxy

	safe.Go("bluetooth scan", b.settle)
	safe.Go("bluetooth reports", b.deliver)
	return b
}

func (b *Proxy) Name() string { return "bluetooth proxy" }

func (b *Proxy) Entities() []esphome.Entity { return []esphome.Entity{b.enable} }

// Handle answers the proxy's own protocol messages: subscribe, unsubscribe, set mode.
func (b *Proxy) Handle(ctx context.Context, conn *esphome.Conn, msg proto.Message) error {
	return b.proxy.Handle(ctx, conn, msg)
}

// Start brings the scan in line once the radio is up, which is the phase before this one.
func (b *Proxy) Start(context.Context) error {
	b.apply()
	return nil
}

// Close stops the scan, so a restart does not leave the controller listening for nobody.
func (b *Proxy) Close() error {
	b.halt()
	return nil
}

// Enabled reports whether the user has asked for the proxy.
func (b *Proxy) Enabled() bool { return config.Get().Bluetooth.Proxy }

// SetProxy turns the proxy on or off. Home Assistant's switch and the harness both come here.
func (b *Proxy) SetProxy(on bool) {
	b.enable.Set(on)
	if err := config.Set().Bluetooth().Proxy(on); err != nil {
		slog.Error("saving the bluetooth proxy setting failed", "err", err)
		return
	}
	b.apply()

	// What the device advertises has changed, and that is only read when a client connects.
	component.Reconnect.Emit(struct{}{})
}

// Dropped is how many reports Home Assistant was too slow to take, for diagnostics.
func (b *Proxy) Dropped() uint64 { return b.dropped.Load() }

// Advertise is what to tell Home Assistant this device can do, and nothing while the proxy is off.
//
// Not advertising is the only way to say it is not a proxy, and it costs a reconnect since device
// info is read once per connection. Reporting a stopped scanner instead makes Home Assistant read
// the requested and current modes as disagreeing and ask for a power cycle.
func (b *Proxy) Advertise() esphome.BluetoothFeature {
	if b.Enabled() {
		return Features
	}
	return 0
}

// apply asks for the scan to be brought in line, without waiting for it.
func (b *Proxy) apply() {
	select {
	case b.wanted <- struct{}{}:
	default:
	}
}

// settle serialises scan changes, so two callers cannot fight over the one scanner.
func (b *Proxy) settle() {
	for range b.wanted {
		b.tune()
	}
}

// tune starts or stops the scan to match what is wanted.
//
// It follows the switch, not the subscription: Home Assistant keeps its requested mode across a
// reconnect, so a stopped scanner in between reads as a broken one. A mode change is a restart.
func (b *Proxy) tune() {
	want := b.Enabled()
	active := b.proxy.Active()

	b.mu.Lock()
	settled := want == b.running && (!want || active == b.active)
	b.mu.Unlock()

	if settled {
		return
	}

	b.halt()
	if !want {
		_ = b.proxy.Report(esphome.ScannerStopped)
		return
	}

	if !b.radio.Up() {
		slog.Warn("bluetooth proxy asked for but the radio is not up")
		_ = b.proxy.Report(esphome.ScannerFailed)
		return
	}

	ctx, stop := context.WithCancel(context.Background())

	b.mu.Lock()
	b.stop, b.running, b.active = stop, true, active
	b.mu.Unlock()

	safe.Go("bluetooth scanning", func() {
		defer stop()

		err := b.radio.Scan(ctx, active, b.found)

		b.mu.Lock()
		b.running = false
		b.mu.Unlock()

		// One that ends on its own has taken the proxy with it, so say so rather than leave Home
		// Assistant reporting a scanner that is running.
		if ctx.Err() == nil {
			slog.Error("bluetooth scanning stopped", "err", err)
			_ = b.proxy.Report(esphome.ScannerFailed)
			b.apply()
		}
	})

	slog.Info("bluetooth scanning", "active", active)
	_ = b.proxy.Report(esphome.ScannerRunning)
}

// halt stops whatever scan is running. No wait: the scanner turns itself off on the way out.
func (b *Proxy) halt() {
	b.mu.Lock()
	stop := b.stop
	b.stop, b.running = nil, false
	b.mu.Unlock()

	if stop != nil {
		stop()
	}
}

// found runs on the scanner's goroutine. The scan stays up to hold its Home Assistant state, but
// nothing has anywhere to go before a client subscribes.
func (b *Proxy) found(a ble.Advertisement) {
	if !b.proxy.Subscribed() {
		return
	}
	b.queue(a)
}

// queue hands one report to the deliverer, or drops it.
//
// Never blocks. It runs on the goroutine reading the controller; stalling it overflows the
// controller's queue, and the driver then discards packets for everything on the chip, wifi too.
func (b *Proxy) queue(a ble.Advertisement) {
	select {
	case b.reports <- a:
	default:
		if n := b.dropped.Add(1); n%1000 == 1 {
			slog.Warn("bluetooth reports dropped, home assistant is behind", "total", n)
		}
	}
}

// deliver batches what the radio heard and hands it over.
func (b *Proxy) deliver() {
	var pending []*api.BluetoothLERawAdvertisement

	flush := time.NewTimer(hold)
	defer flush.Stop()
	flush.Stop()

	send := func() {
		if len(pending) == 0 {
			return
		}
		ads := pending
		pending = nil

		if err := b.proxy.Advertise(ads); err != nil {
			slog.Debug("bluetooth advertisements not delivered", "count", len(ads), "err", err)
		}
	}

	for {
		select {
		case a := <-b.reports:
			pending = append(pending, &api.BluetoothLERawAdvertisement{
				Address:     a.Addr(),
				Rssi:        int32(a.RSSI),
				AddressType: uint32(a.AddressType),
				Data:        a.Data,
			})
			if len(pending) == 1 {
				flush.Reset(hold)
			}
			if len(pending) >= batch {
				flush.Stop()
				send()
			}

		case <-flush.C:
			send()
		}
	}
}
