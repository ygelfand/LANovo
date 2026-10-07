package config

import (
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

// Feedback is the noises the device makes about itself, as opposed to anything it was asked to
// play.
type Feedback struct {
	// Chime is which sound an acknowledgement makes, and whether it makes one at all.
	Chime Chime `json:"chime"`
}

// DefaultChime is the short rising pair, which is what the device has always done.
const DefaultChime = ChimeChirp

func defaultFeedback() Feedback { return Feedback{Chime: DefaultChime} }

type Chime = schema.Chime

const (
	ChimeNone  = schema.ChimeNone
	ChimeChirp = schema.ChimeChirp
	ChimeDing  = schema.ChimeDing
	ChimeRise  = schema.ChimeRise
)

var Chimes = schema.Chimes

// FeedbackWriter changes the noises the device makes about itself.
type FeedbackWriter struct{ st *Store }

func (w FeedbackWriter) Chime(v Chime) error {
	return w.st.Update(func(c *Config) { c.Feedback.Chime = v })
}
