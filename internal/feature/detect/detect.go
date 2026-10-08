package detect

import (
	"context"
	"log/slog"
	"sync"
	"time"

	sharedengine "github.com/ygelfand/libcountertop/pkg/inference/detect"

	"github.com/ygelfand/LANovo/internal/layout"

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
	component.Register(component.Device, Get, component.Order(40),
		component.Supervise(service.Restart(time.Second, 30*time.Second)))
}

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

	e.Load = func() error {
		turn := voice.Get()
		turn.SetSlots(d.load(turn.Slots()))
		d.stop.Load()
		return nil
	}

	voice.Get().OnWakeWord(d.load)

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

func (d *Detect) fired(slot int) {
	if slot == StopSlot {
		voice.Get().Interrupt()
		return
	}
	voice.Get().Start(slot)
}

func (d *Detect) load(ids []string) []string {
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

	if gone, freed := wake.Lib().Purge(accepted); gone > 0 {
		slog.Info("wake words dropped", "count", gone, "freed", freed)
	}

	return accepted
}
