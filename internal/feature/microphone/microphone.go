package microphone

import (
	"fmt"
	"strconv"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/say"
	setting "github.com/ygelfand/libcountertop/pkg/settings"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
)

func init() {
	component.Register(sharedcomponent.Device, Get)
}

// The codec's decimator volume tops out at +40 dB.
const GainMost = mic.MaxGain - config.DefaultMicGain

const (
	SensitivityLeast = 4
	SensitivityMost  = 20
)

const Group setting.Group = "Microphone"

type Setting = setting.Setting[config.Microphone]

type Microphone struct {
	knobs *setting.Controls[config.Microphone]
}

var (
	once   sync.Once
	shared *Microphone
)

func Get() *Microphone {
	once.Do(func() {
		shared = &Microphone{knobs: &setting.Controls[config.Microphone]{
			Table:  Table(),
			Device: component.DeviceMicrophone,
			Read:   func() config.Microphone { return config.Get().Microphone },
			Save:   store,
		}}
		component.Settings.Add(shared.knobs)
	})
	return shared
}

func Table() *setting.Table[config.Microphone] {
	return setting.NewTable("microphone", []setting.Group{Group}, []Setting{
		{
			Name:  "gain",
			Group: Group,
			Icon:  "mdi:volume-plus",
			Kind:  setting.Number,
			Max:   GainMost,
			Unit:  "dB",
			Read: func(m *config.Microphone) string {
				return strconv.Itoa(m.Gain - config.DefaultMicGain)
			},
			Write: func(m *config.Microphone, v string) error {
				n, err := strconv.Atoi(v)
				if err != nil || n < 0 || n > GainMost {
					return fmt.Errorf("gain takes 0 to %d dB, not %q", GainMost, v)
				}
				m.Gain = config.DefaultMicGain + n
				return nil
			},
		},
		setting.Flag(
			Setting{Name: "leveling", Group: Group, Icon: "mdi:signal-variant"},
			func(m *config.Microphone) *bool { return &m.Leveling },
		),
		setting.Flag(
			Setting{Name: "denoise", Group: Group, Icon: "mdi:waveform"},
			func(m *config.Microphone) *bool { return &m.Denoise },
		),
		setting.Count(
			Setting{
				Name:  "sensitivity",
				Group: Group,
				Icon:  "mdi:motion-sensor",
				Min:   SensitivityLeast,
				Max:   SensitivityMost,
				Unit:  "dB",
			},
			func(m *config.Microphone) *int { return &m.Sensitivity },
		),
	}, setting.Messages{Text: say.T, Missing: say.Missing})
}

var applies = map[string]func(config.Microphone) error{
	"gain": func(m config.Microphone) error { return mic.Get().SetGain(m.Gain) },
	"leveling": func(m config.Microphone) error {
		mic.Get().SetLeveling(m.Leveling)
		return nil
	},
	"denoise": func(m config.Microphone) error {
		mic.Get().SetDenoising(m.Denoise)
		return nil
	},
	"sensitivity": func(m config.Microphone) error {
		mic.Get().SetSensitivity(m.Sensitivity)
		return nil
	},
}

func store(s Setting, v string) error {
	if err := setting.Store(config.MicrophoneSection, s, v); err != nil {
		return err
	}
	return applies[s.Name](config.Get().Microphone)
}

func (m *Microphone) Name() string { return "microphone settings" }

func (m *Microphone) Entities() []esphome.Entity { return m.knobs.Entities() }
