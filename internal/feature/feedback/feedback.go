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

func (output) Chime(notes []sharedtone.Note) { play(notes) }

var play = func(notes []Note) { speaker.Sound().Chime(notes) }

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

func Volume() { sound(sharedtone.Wake(config.Get().Feedback.Chime)) }

func Muted() { sound(sharedtone.ToneMute) }

func Unmuted() { sound(sharedtone.ToneUnmute) }
