package speaker

import (
	"github.com/ygelfand/libcountertop/pkg/audio/mix"
	"github.com/ygelfand/libcountertop/pkg/audio/tone"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

func (s *Speaker) Beep(freq float64, ms int, level float64) {
	s.Chime(schema.StreamFeedback, level, tone.Note{Freq: freq, Ms: ms})
}

func (s *Speaker) Chime(stream schema.Stream, level float64, notes ...tone.Note) {
	s.Overlay(stream, mix.Chime(Rate, Channels, level, notes...))
}
