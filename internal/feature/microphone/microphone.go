package microphone

import (
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
)

func init() {
	component.Register(component.Device, Get)
}

// The codec's decimator volume tops out at +40 dB.
const GainMost = mic.MaxGain - config.DefaultMicGain

type Microphone struct {
	gain        *esphome.Number
	sensitivity *esphome.Number
	lift        *esphome.Number
}

var (
	once   sync.Once
	shared *Microphone
)

func Get() *Microphone {
	once.Do(func() { shared = build() })
	return shared
}

func build() *Microphone {
	m := &Microphone{
		gain: &esphome.Number{
			Base: esphome.Base{
				ObjectID: "microphone_gain",
				Name:     "Microphone gain",
				Icon:     "mdi:volume-plus",
				Category: esphome.CategoryConfig,
			},
			Min: 0, Max: GainMost, Step: 1, Unit: "dB",
			Mode: esphome.NumberBox,
		},
		sensitivity: &esphome.Number{
			Base: esphome.Base{
				ObjectID: "microphone_sensitivity",
				Name:     "Room sensitivity",
				Icon:     "mdi:motion-sensor",
				Category: esphome.CategoryConfig,
			},
			Min: 4, Max: 20, Step: 1, Unit: "dB",
			Mode: esphome.NumberBox,
		},
	}
	m.lift = &esphome.Number{
		Base: esphome.Base{
			ObjectID: "visualizer_lift",
			Name:     "Visualizer lift",
			Icon:     "mdi:equalizer",
			Category: esphome.CategoryConfig,
		},
		Min: 0, Max: visuals.LiftMax, Step: 1, Unit: "dB",
		Mode: esphome.NumberBox,
	}
	for _, b := range []*esphome.Base{&m.gain.Base, &m.sensitivity.Base, &m.lift.Base} {
		b.DeviceID = component.DeviceMicrophone
	}

	cfg := config.Get().Microphone
	m.gain.Set(float32(cfg.Gain - config.DefaultMicGain))
	m.sensitivity.Set(float32(cfg.Sensitivity))
	m.lift.Set(float32(cfg.VisualizerLift))
	m.lift.OnCommand = func(v float32) { m.SetLift(int(v)) }

	m.gain.OnCommand = func(v float32) { m.SetGain(int(v)) }
	m.sensitivity.OnCommand = func(v float32) { m.SetSensitivity(int(v)) }
	return m
}

func (m *Microphone) Name() string { return "microphone settings" }

func (m *Microphone) Entities() []esphome.Entity {
	return []esphome.Entity{m.gain, m.sensitivity, m.lift}
}

func (m *Microphone) SetLift(db int) {
	db = min(max(db, 0), visuals.LiftMax)
	m.lift.Set(float32(db))
	visuals.Get().SetLift(db)
	if err := config.Set().Microphone().VisualizerLift(db); err != nil {
		slog.Error("saving the visualizer lift failed", "err", err)
	}
}

func (m *Microphone) SetGain(db int) {
	db = min(max(db, 0), GainMost)
	m.gain.Set(float32(db))
	if err := mic.Get().SetGain(config.DefaultMicGain + db); err != nil {
		slog.Error("setting the microphone gain failed", "err", err)
	}
	if err := config.Set().Microphone().Gain(config.DefaultMicGain + db); err != nil {
		slog.Error("saving the microphone gain failed", "err", err)
	}
	slog.Info("microphone gain", "db", db)
}

func (m *Microphone) SetLeveling(on bool) {
	mic.Get().SetLeveling(on)
	if err := config.Set().Microphone().Leveling(on); err != nil {
		slog.Error("saving the microphone leveling failed", "err", err)
	}
}

func (m *Microphone) SetDenoising(on bool) {
	mic.Get().SetDenoising(on)
	if err := config.Set().Microphone().Denoise(on); err != nil {
		slog.Error("saving the noise suppression failed", "err", err)
	}
}

func (m *Microphone) SetSensitivity(db int) {
	db = min(max(db, 4), 20)
	m.sensitivity.Set(float32(db))
	mic.Get().SetSensitivity(db)
	if err := config.Set().Microphone().Sensitivity(db); err != nil {
		slog.Error("saving the room sensitivity failed", "err", err)
	}
}
