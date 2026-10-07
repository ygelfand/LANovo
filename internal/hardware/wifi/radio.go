// Package wifi owns the radio: the driver, the interface, and the supplicant that associates.
//
// Addresses belong to the dhcp feature.
package wifi

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/service"
	"github.com/ygelfand/libcountertop/pkg/hook"
)

func init() {
	component.Register(component.Hardware, Get, component.Order(20),
		component.Supervise(service.Restart(2*time.Second, time.Minute)))
}

// Radio is the wireless interface.
type Radio struct {
	// Associated carries every change in whether the supplicant is on a network.
	Associated hook.Hook[bool]

	mu   sync.Mutex
	mac  string
	up   bool
	ssid string

	// ctl is the control connection Run holds, for asking what the supplicant is doing.
	ctl *Control
}

var (
	once   sync.Once
	shared *Radio
)

// Get is the radio. Nothing here touches hardware: Start does.
func Get() *Radio { once.Do(func() { shared = &Radio{} }); return shared }

func (r *Radio) Name() string { return "wifi" }

// MAC is the interface's address, which is the device's identity to Home Assistant.
func (r *Radio) MAC() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.mac
}

// Startup is not ready until the radio has associated: running means the supplicant is up, which
// it is long before it has joined anything.
func (r *Radio) Startup() component.Progress {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := component.Progress{Done: r.up}
	switch {
	case r.up:
		p.Doing = "on " + r.ssid
	case r.ssid != "":
		p.Doing = "joining " + r.ssid
	default:
		p.Doing = "looking for the network"
	}
	return p
}

// Network is the network the supplicant is on, empty when it is on none.
func (r *Radio) Network() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.up {
		return ""
	}
	return r.ssid
}

// Start loads the driver and brings the interface up. The supplicant is init's, and Run is what
// talks to it.
func (r *Radio) Start(context.Context) error {
	if err := Load(); err != nil {
		return err
	}
	if err := linkUp(Interface); err != nil {
		return err
	}

	mac, err := MAC()
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.mac = mac
	r.mu.Unlock()

	return nil
}

// Run follows the supplicant until ctx is canceled.
//
// init restarts a supplicant that dies, so this runs again against a new one: it waits for the
// socket and asks it to join, both of which a supplicant that has just started needs.
func (r *Radio) Run(ctx context.Context) error {
	if err := AwaitSupplicant(); err != nil {
		return err
	}

	if err := SetPowerSave(Interface, false); err != nil {
		slog.Warn("could not turn power save off", "err", err)
	}

	// Always, not only when disconnected: power save takes at the next association, and init's
	// supplicant may have joined before this ran.
	if err := Connect(); err != nil {
		return err
	}

	// One connection for the life of the run. Dialling per question means a unix socket created,
	// chmodded, connected, closed and unlinked every time the supplicant says anything, which on
	// a talkative supplicant is thousands of files a second.
	ctl, err := Dial()
	if err != nil {
		return err
	}
	defer ctl.Close()

	r.mu.Lock()
	r.ctl = ctl
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.ctl = nil
		r.mu.Unlock()
		r.apply(false, "")
	}()

	// Subscribe before the snapshot so a completed association cannot be missed.
	return listen(ctx, r.settle, func(e Event) {
		switch {
		case e.Is(EventConnected):
			r.connected()
		case e.Is(EventStateChange):
			if s, ok := state(e.Text); ok {
				r.apply(s.Completed, s.SSID)
			}
		case e.Is(EventDisconnected):
			r.apply(false, "")
		case e.Is(EventTerminating):
			r.apply(false, "")
			slog.Warn("the supplicant is going away")
		}
	})
}

// settle asks the supplicant what it is doing, for the start of a run: the first state change may
// be a long way off, and the device may already be associated.
//
// Only from there. Answering STATUS sends the supplicant back through set_state, which emits the
// state change again, so asking in response to one never stops.
func (r *Radio) settle() {
	r.mu.Lock()
	ctl := r.ctl
	r.mu.Unlock()

	if ctl == nil {
		return
	}

	s, err := ctl.Status()
	if err != nil {
		slog.Warn("could not read the supplicant's status", "err", err)
		return
	}
	r.apply(s["wpa_state"] == "COMPLETED", s["ssid"])
}

// connected follows completion when a vendor omits STATE-CHANGE.
// Query only on a new association: vendor STATUS replies can emit CONNECTED again.
func (r *Radio) connected() {
	r.mu.Lock()
	ctl, ssid, already := r.ctl, r.ssid, r.up
	r.mu.Unlock()
	if already {
		return
	}
	// Mark the event handled before querying, so a repeated completion cannot feed back.
	r.apply(true, ssid)
	if ctl != nil {
		if status, err := ctl.Status(); err == nil && status["ssid"] != "" {
			r.apply(true, status["ssid"])
		}
	}
}

// apply records what the radio is doing and says so if it changed.
func (r *Radio) apply(up bool, ssid string) {
	r.mu.Lock()
	changed := up != r.up || ssid != r.ssid
	r.up, r.ssid = up, ssid
	r.mu.Unlock()

	if !changed {
		return
	}
	if up {
		slog.Info("associated", "network", ssid)
	} else {
		slog.Warn("not associated")
	}
	r.Associated.Emit(up)
}
