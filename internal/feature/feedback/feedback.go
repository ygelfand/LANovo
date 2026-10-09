package feedback

import (
	"sync"

	sharedtone "github.com/ygelfand/libcountertop/pkg/audio/tone"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

type Feedback struct{ *sharedtone.Feedback }

func (f *Feedback) Name() string          { return "feedback" }
func (f *Feedback) Restore(config.Config) { f.Feedback.Restore() }

type output struct{}

func (output) Chime(s config.Stream, notes []sharedtone.Note) { play(s, notes) }

var play = func(s config.Stream, notes []Note) { speaker.Sound().Chime(s, notes) }

type Note = sharedtone.Note

var get = sync.OnceValue(func() *Feedback {
	return &Feedback{
		sharedtone.NewFeedback(config.FeedbackSection, output{}, component.DevicePlayback),
	}
})

func Get() *Feedback { return get() }

func init()              { component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(15)) }
func sound(notes []Note) { Get().Sound(notes) }

func Failure() { sound(sharedtone.ToneTrouble) }

func Canceled() { sound(sharedtone.ToneCancel) }

func Preview(s config.Stream) { Get().Preview(s) }

func Muted() { sound(sharedtone.ToneMute) }

func Unmuted() { sound(sharedtone.ToneUnmute) }
