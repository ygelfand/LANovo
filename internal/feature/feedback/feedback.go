package feedback

import (
	"sync"

	sharedtone "github.com/ygelfand/libcountertop/pkg/audio/tone"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/tones"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

type Feedback struct{ *sharedtone.Feedback }

func (f *Feedback) Name() string          { return "feedback" }
func (f *Feedback) Restore(config.Config) { f.Feedback.Restore() }

type output struct{}

func (output) Chime(s config.Stream, notes []sharedtone.Note) { play(s, notes) }

func (output) Sound(s config.Stream, level float64, snd sharedtone.Sound) {
	if len(snd.Clip) == 0 {
		play(s, snd.Notes)
		return
	}
	speaker.Sound().Sound(s, level, snd)
}

var play = func(s config.Stream, notes []Note) { speaker.Sound().Chime(s, notes) }

type Note = sharedtone.Note

var get = sync.OnceValue(func() *Feedback {
	f := &Feedback{
		sharedtone.NewFeedback(config.FeedbackSection, output{}, component.DevicePlayback,
			sharedtone.FeedbackOptions{Sounds: tones.Get(), Mute: true}),
	}
	component.Settings.Add(f.Controls)
	tones.Get().Changed.Listen(func(struct{}) { f.Refresh() })
	return f
})

func Get() *Feedback { return get() }

func init() { component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(15)) }

func Failure() { Get().Failure() }

func Canceled() { Get().Cancelled() }

func Preview(s config.Stream) { Get().Preview(s) }

func Muted() { Get().Muted() }

func Unmuted() { Get().Unmuted() }
