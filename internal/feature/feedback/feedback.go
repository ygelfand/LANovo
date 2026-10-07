package feedback

import (
	"sync"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	sharedtone "github.com/ygelfand/libcountertop/pkg/audio/tone"
)

type Feedback struct{ *sharedtone.Feedback }

func (f *Feedback) Name() string          { return "feedback" }
func (f *Feedback) Restore(config.Config) { f.Feedback.Restore() }

type output struct{}

func (output) Chime(notes []sharedtone.Note) { play(notes) }

var play = func(notes []Note) { speaker.Sound().Chime(notes) }

type Note = sharedtone.Note

var get = sync.OnceValue(func() *Feedback {
	return &Feedback{sharedtone.NewFeedback(config.FeedbackSection, output{}, component.DevicePlayback)}
})

func Get() *Feedback     { return get() }
func init()              { component.Register(component.Device, Get, component.Order(15)) }
func sound(notes []Note) { Get().Sound(notes) }

// Failure says something did not work: a request that cannot be served has to sound different from
// one that was, or it is indistinguishable from the device having ignored the person.
func Failure() { sound(sharedtone.ToneTrouble) }

// Canceled is a request dropped on purpose, which is neither a failure nor an answer.
func Canceled() { sound(sharedtone.ToneCancel) }

// Volume acknowledges a level moving, for the presses a screen is not being watched for.
//
// The one occasion whose sound is chosen. The others carry meaning in their shape — rising for on,
// falling twice for trouble — and a setting that overrode those would be choosing what the device
// is allowed to say rather than how it says it.
func Volume() { sound(sharedtone.Wake(config.Get().Feedback.Chime)) }

// Muted and Unmuted are the microphones being cut and restored. Rising for on, falling for off, so
// which way it went is audible without looking.
func Muted() { sound(sharedtone.ToneMute) }

func Unmuted() { sound(sharedtone.ToneUnmute) }
