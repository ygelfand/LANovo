package visuals

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/display/visualinput"
	"github.com/ygelfand/libcountertop/pkg/display/visualview"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/gpu"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

const LiftMax = visualinput.LiftMax

type Visuals struct{ *visualinput.Visuals }

func init() { component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(60)) }

var get = sync.OnceValue(func() *Visuals {
	v := &Visuals{visualinput.New(visualinput.Options{
		Settings:        config.VisualSection,
		MicRate:         16000,
		MicChannels:     2,
		SpeakerRate:     speaker.Rate,
		SpeakerChannels: speaker.Channels,
		Lift:            config.LiftSection,
		DeviceID:        component.DeviceScreen,
		Listen:          func() (<-chan []int16, func()) { return mic.Get().ListenStereo("visuals") },
		Muted:           func() bool { return privacy.Get().MicMuted() },
		Tap:             func(t visualinput.Tap) { speaker.Get().SetTap(t) },
	})}
	component.Settings.Add(v.Settings())
	return v
})

func Get() *Visuals { return get() }

func (v *Visuals) Restore(c config.Config) { v.Visuals.Restore(c.Visual) }

func (v *Visuals) View() *visualview.View {
	return visualview.New(v, visualview.Dependencies{
		GPU:      gpu.Get(),
		Display:  display.Get(),
		Shell:    shell.Get(),
		Settings: config.VisualSection,
	})
}
