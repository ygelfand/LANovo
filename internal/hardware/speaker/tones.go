package speaker

import (
	"github.com/ygelfand/libcountertop/pkg/audio/mix"
	"github.com/ygelfand/libcountertop/pkg/audio/tone"
)

func (s *Speaker) Beep(freq float64, ms int, level float64) {
	s.Chime(level, tone.Note{Freq: freq, Ms: ms})
}

func (s *Speaker) Chime(level float64, notes ...tone.Note) {
	s.Overlay(mix.Chime(Rate, Channels, level, notes...))
}
