package config

import "github.com/ygelfand/libcountertop/pkg/say"

// Feedback is the noises the device makes about itself, as opposed to anything it was asked to
// play.
type Feedback struct {
	// Chime is which sound an acknowledgement makes, and whether it makes one at all.
	Chime Chime `json:"chime"`
}

// DefaultChime is the short rising pair, which is what the device has always done.
const DefaultChime = ChimeChirp

func defaultFeedback() Feedback { return Feedback{Chime: DefaultChime} }

// Chime is what the device sounds like when it acknowledges something.
//
// A choice rather than a switch, because the device sits in a room with other devices in it and a
// beep that is distinguishable from the microwave is worth more than a beep. None is one of the
// answers: a display on a bedside table should be able to say nothing at all.
type Chime string

const (
	// ChimeNone is silence. It covers every sound the device makes about itself, not only the
	// acknowledgement, because somebody who turned the chimes off wants the device quiet rather
	// than quieter.
	ChimeNone Chime = "none"

	// ChimeChirp is two quick rising notes.
	ChimeChirp Chime = "chirp"

	// ChimeDing is one clear note, for a room where the chirp reads as a notification.
	ChimeDing Chime = "ding"

	// ChimeRise is three rising notes, which is the most this should ever be: an acknowledgement
	// longer than a moment is in the way of the thing it is acknowledging.
	ChimeRise Chime = "rise"
)

// Label is how the setting is shown.
func (c Chime) Label() string {
	switch c {
	case ChimeNone:
		return say.T("chime.none")
	case ChimeDing:
		return say.T("chime.ding")
	case ChimeRise:
		return say.T("chime.rise")
	}
	return say.T("chime.chirp")
}

// Chimes is every value the setting takes, in the order they are offered. None first, because it is
// the one somebody goes looking for.
func Chimes() []Chime { return []Chime{ChimeNone, ChimeChirp, ChimeDing, ChimeRise} }

// Silent reports whether the device should make no sound of its own.
func (c Chime) Silent() bool { return c == ChimeNone }

// FeedbackWriter changes the noises the device makes about itself.
type FeedbackWriter struct{ st *Store }

func (w FeedbackWriter) Chime(v Chime) error {
	return w.st.Update(func(c *Config) { c.Feedback.Chime = v })
}
