// Package feedback is how the device tells the person in front of it that something happened.
//
// The occasions are named here — a failure, a request dropped — and each decides for itself what it
// uses. What matters is that the decision is made in one place, so a caller says what happened
// rather than choosing how to say it, and two callers reporting the same thing cannot disagree about
// whether it makes a sound.
//
// Only momentary things live here. An indication that lasts as long as a state belongs to whatever
// owns that state, because ending it is that owner's business.
//
// Every occasion goes through sound, which is the one place the chime setting is read. A caller
// that reached for the speaker directly would be a sound nobody can turn off.
package feedback

import (
	"log/slog"
	"sync"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	esphome "github.com/ygelfand/go-esphome-device"
	sharedtone "github.com/ygelfand/libcountertop/pkg/audio/tone"
)

func init() {
	component.Register(component.Device, Get, component.Order(15))
}

// Feedback owns the setting for what these occasions sound like.
//
// Only the acknowledgement is a choice. The rest is not: the tones are short and few and telling
// them apart is the point of them, so what a failure sounds like is the device's business and
// whether it says anything at all is the user's.
type Feedback struct{ chime *esphome.Select }

var (
	once   sync.Once
	shared *Feedback
)

func Get() *Feedback {
	once.Do(func() {
		shared = &Feedback{
			chime: &esphome.Select{
				Base: esphome.Base{
					ObjectID: "device_chime",
					DeviceID: component.DevicePlayback,
					Name:     "Device chime",
					Icon:     "mdi:bell-ring-outline",
					Category: esphome.CategoryConfig,
				},
				Options: config.Labels(config.Chimes()),
			},
		}

		shared.chime.OnCommand = func(label string) {
			if chime, ok := config.ByLabel(config.Chimes(), label); ok {
				shared.SetChime(chime)
			}
		}
	})
	return shared
}

// SetChime changes what an acknowledgement sounds like and remembers it. Everything that changes it
// comes through here, so Home Assistant is told whatever asked for it.
func (f *Feedback) SetChime(chime config.Chime) {
	f.chime.Set(chime.Label())

	if err := config.Set().Feedback().Chime(chime); err != nil {
		slog.Error("saving a setting failed", "setting", f.chime.ObjectID, "err", err)
	}
}

func (f *Feedback) Name() string { return "feedback" }

func (f *Feedback) Entities() []esphome.Entity { return []esphome.Entity{f.chime} }

// Restore puts the saved choice back, correcting one this build does not have: a name that cannot
// be played would otherwise leave the device silent with no way to tell why.
func (f *Feedback) Restore(c config.Config) {
	chime := c.Feedback.Chime
	if _, ok := config.ByLabel(config.Chimes(), chime.Label()); !ok {
		slog.Warn("no such chime, using the default", "chime", chime)
		chime = config.DefaultChime

		if err := config.Set().Feedback().Chime(chime); err != nil {
			slog.Error("correcting the chime failed", "err", err)
		}
	}
	f.chime.Set(chime.Label())
}

// sound plays a tone unless the device has been told to be quiet.
//
// None silences everything rather than only the acknowledgement: somebody who turned the chimes off
// wants the device quiet, not quieter, and a mute that still beeps at them at two in the morning is
// the setting not working.
func sound(notes []Note) {
	if config.Get().Feedback.Chime.Silent() {
		return
	}
	play(notes)
}

// play is the speaker, as a variable so a test can hold what the gate lets through without a sound
// card. The occasions are the thing worth testing and they are all silent to a test otherwise.
var play = func(notes []Note) { speaker.Sound().Chime(notes) }

// Note is a tone, re-exported so an occasion can name one without every caller importing the
// speaker.
type Note = speaker.Note

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
