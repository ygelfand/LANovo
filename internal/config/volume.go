package config

import (
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

type Volume struct {
	Media    int `json:"media"`
	Alerts   int `json:"alerts"`
	Voice    int `json:"voice"`
	Feedback int `json:"feedback"`
}

const (
	DefaultMediaVolume    = 40
	DefaultAlertsVolume   = 60
	DefaultVoiceVolume    = 50
	DefaultFeedbackVolume = 30
)

func defaultVolume() Volume {
	return Volume{
		Media:    DefaultMediaVolume,
		Alerts:   DefaultAlertsVolume,
		Voice:    DefaultVoiceVolume,
		Feedback: DefaultFeedbackVolume,
	}
}

func (v Volume) Level(s Stream) int {
	switch s {
	case StreamAlerts:
		return v.Alerts
	case StreamVoice:
		return v.Voice
	case StreamFeedback:
		return v.Feedback
	}
	return v.Media
}

type Stream = schema.Stream

const (
	StreamMain     = schema.StreamMain
	StreamMedia    = schema.StreamMedia
	StreamAlerts   = schema.StreamAlerts
	StreamVoice    = schema.StreamVoice
	StreamFeedback = schema.StreamFeedback
)

func Streams() []Stream {
	return []Stream{StreamMedia, StreamAlerts, StreamVoice, StreamFeedback}
}

type VolumeWriter struct{ st *Store }

func (w VolumeWriter) Level(s Stream, v int) error {
	return w.st.Update(func(c *Config) {
		switch s {
		case StreamAlerts:
			c.Volume.Alerts = v
		case StreamVoice:
			c.Volume.Voice = v
		case StreamFeedback:
			c.Volume.Feedback = v
		default:
			c.Volume.Media = v
		}
	})
}
