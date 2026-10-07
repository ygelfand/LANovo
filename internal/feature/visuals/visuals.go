package visuals

import (
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/privacy"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	sharedinput "github.com/ygelfand/libcountertop/pkg/display/visualinput"
	"sync"
)

const LiftMax = sharedinput.LiftMax
const LabelMost = sharedinput.LabelMost

type Visuals struct{ *sharedinput.Visuals }

func init() { component.Register(component.Device, Get, component.Order(60)) }

var get = sync.OnceValue(func() *Visuals {
	return newVisuals(
		func() (<-chan []int16, func()) { return mic.Get().ListenStereo("visuals") }, func() bool { return privacy.Get().MicMuted() },
		speaker.Get().SetTap,
	)
})

func Get() *Visuals { return get() }
func newVisuals(listen func() (<-chan []int16, func()), muted func() bool, tap func(speaker.Tap)) *Visuals {
	return &Visuals{sharedinput.New(sharedinput.Options{
		Read:    func() config.Visual { return config.Get().Visual },
		MicRate: 16000, SpeakerRate: speaker.Rate, SpeakerChannels: speaker.Channels, Lift: config.Get().Microphone.VisualizerLift, DeviceID: component.DeviceScreen,
		Listen: listen, Muted: muted, Tap: func(t sharedinput.Tap) { tap(t) },
		Kind:   func(v string) error { return config.Set().Visual().Kind(v) },
		Label:  func(v string) error { return config.Set().Visual().Label(v) },
		Seed:   func(v int) error { return config.Set().Visual().Seed(v) },
		MaxFPS: func(v int) error { return config.Set().Visual().MaxFPS(v) },
	})}
}
func (v *Visuals) Restore(c config.Config) { v.Visuals.Restore(c.Visual) }
