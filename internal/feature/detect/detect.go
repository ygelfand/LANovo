package detect

import (
	"sync"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/inference/detect"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/service"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/call"
	"github.com/ygelfand/LANovo/internal/feature/diag"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/voice"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/lib/wake"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(40),
		sharedcomponent.Supervise(service.Restart(time.Second, 30*time.Second)))
}

type Detect struct {
	*detect.Engine
	stop *detect.StopWord
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
	queue := detect.NewQueue(8)
	e := detect.New(detect.StopSlot+1, queue)

	e.Threshold = func(slot int) float64 {
		if slot == detect.StopSlot {
			return config.Get().Wake.Stop.Threshold
		}
		return wakeword.Threshold(slot)
	}
	e.OnDetect = func(slot int) {
		if slot == detect.StopSlot {
			voice.Get().Interrupt()
			return
		}
		voice.Get().Start(slot)
	}

	d := &Detect{Engine: e}
	d.stop = detect.NewStopWord(e, detect.StopOptions{
		Directory: layout.StateDir, DeviceID: component.DeviceMicrophone,
		Read: func() float64 { return config.Get().Wake.Stop.Threshold },
		Save: func(v float64) error { return config.Set().Stop().Threshold(v) },
	})

	e.Load = func() error {
		turn := voice.Get()
		turn.SetSlots(d.load(turn.Slots()))
		d.stop.Load()
		return nil
	}

	voice.Get().OnWakeWord(d.load, diag.Get().Measure)

	mic.Get().Speech.Listen(func(f mic.Frame) {
		if privacy.Get().MicMuted() || call.Get().PausesWake() {
			return
		}
		queue.Feed(f.Samples)
	})

	detect.LogInstalled(wake.Lib().Ours())
	return d
}

func (d *Detect) Entities() []esphome.Entity { return []esphome.Entity{d.stop.Entity} }
func (d *Detect) Restore(c config.Config)    { d.stop.Restore(c.Wake.Stop.Threshold) }

func (d *Detect) load(ids []string) []string {
	accepted, _ := detect.Choose(d.Engine, wake.Lib().Ensure(ids), ids, wakeword.Slots)
	return accepted
}
