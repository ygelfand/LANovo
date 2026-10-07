package config

import (
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

type Feedback = schema.Feedback

const DefaultChime = schema.DefaultChime

var defaultFeedback = schema.DefaultFeedback

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
