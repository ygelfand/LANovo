package detect

import (
	"context"
	"github.com/ygelfand/LANovo/internal/layout"
	sharedengine "github.com/ygelfand/libcountertop/pkg/inference/detect"
	"log/slog"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/voice"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/lib/wake"
	"github.com/ygelfand/LANovo/internal/service"
)

func init() {
	// Before the API, so Home Assistant cannot read the wake words while they are still loading and
	// be told about one that then fails.
	component.Register(component.Device, Get, component.Order(40),
		component.Supervise(service.Restart(time.Second, 30*time.Second)))
}

// Detect is the engine as the device runs it: the wake words the user chose, loaded into it, and
// the turn that follows when one of them fires.
type Detect struct {
	engine *Engine
	stop   *sharedengine.StopWord
}

var (
	once   sync.Once
	shared *Detect
)

func Get() *Detect {
	once.Do(func() { shared = newDetect() })
	return shared
}

func newDetect() *Detect {
	// Sized to reach the stop word's reserved index. The slots between it and Home Assistant's are
	// never loaded, and an unloaded slot is one comparison a frame.
	e := New(StopSlot + 1)

	e.Threshold = func(slot int) float64 {
		if slot == StopSlot {
			return config.Get().Wake.Stop.Threshold
		}
		return wakeword.Threshold(slot)
	}

	d := &Detect{engine: e}
	d.stop = sharedengine.NewStopWord(e, sharedengine.StopOptions{
		Directory: layout.StateDir, DeviceID: component.DeviceMicrophone,
		Read: func() float64 { return config.Get().Wake.Stop.Threshold },
		Save: func(v float64) error { return config.Set().Stop().Threshold(v) },
	})

	e.OnDetect = d.fired

	// The engine loads on every start, including a restart. Home Assistant only pushes a selection
	// when the user changes one, so an engine that came back empty would leave the device deaf while
	// it went on advertising wake words it was not listening for.
	e.Load = func() error {
		turn := voice.Get()
		turn.SetSlots(d.load(turn.Slots()))
		d.stop.Load()
		return nil
	}

	// A selection downloads models and lets go of the ones it replaced, so it is the one thing that
	// moves what the device has on disk.
	voice.Get().OnWakeWord(d.load)

	// A muted microphone is not scored at all: the slider is the one thing in this device that has
	// to mean what it says.
	mic.Get().Speech.Listen(func(f mic.Frame) {
		if privacy.Get().MicMuted() || call.Get().PausesWake() {
			return
		}
		e.Feed(f.Samples)
	})

	ours := wake.Lib().Ours()
	slog.Info("wake words installed", "count", len(ours),
		"openwakeword", len(wake.OfKind(ours, wake.KindOpenWakeWord)),
		"microwakeword", len(wake.OfKind(ours, wake.KindMicroWakeWord)))

	return d
}

func (d *Detect) Name() string { return "wake" }

func (d *Detect) Start(ctx context.Context) error { return d.engine.Start(ctx) }

func (d *Detect) Run(ctx context.Context) error { return d.engine.Run(ctx) }

func (d *Detect) Close() error { return d.engine.Close() }

// fired is a detection, off the audio path. The stop word is not one of Home Assistant's slots and
// means something else entirely, so it goes its own way before any of this.
func (d *Detect) fired(slot int) {
	if slot == StopSlot {
		voice.Get().Interrupt()
		return
	}
	voice.Get().Start(slot)
}

// load puts one wake word in each slot and reports the ids that came up. Whatever the engine refuses
// is left out, so Home Assistant reverts that slot rather than showing a wake word the device is not
// listening for.
func (d *Detect) load(ids []string) []string {
	// A selection may name a model Home Assistant is offering but this device has never had, so the
	// library is asked rather than a list captured at boot: this is where a new word arrives.
	models := wake.Lib().Ensure(ids)

	accepted := make([]string, 0, wakeword.Slots)
	for slot := range wakeword.Slots {
		if slot >= len(ids) || ids[slot] == "" {
			d.engine.Clear(slot)
			accepted = append(accepted, "")
			continue
		}

		m, ok := wake.Find(models, ids[slot])
		if !ok {
			slog.Warn("a chosen wake word is not on the device", "slot", slot+1, "id", ids[slot])
			d.engine.Clear(slot)
			accepted = append(accepted, "")
			continue
		}

		if err := d.engine.Use(slot, m); err != nil {
			slog.Error("loading a wake word failed", "slot", slot+1, "id", m.ID, "err", err)
			d.engine.Clear(slot)
			accepted = append(accepted, "")
			continue
		}
		accepted = append(accepted, m.ID)
	}

	// Nothing keeps a model no slot is listening for: they are a megabyte each and the device
	// fetches one again in seconds.
	if gone, freed := wake.Lib().Purge(accepted); gone > 0 {
		slog.Info("wake words dropped", "count", gone, "freed", freed)
	}

	return accepted
}
