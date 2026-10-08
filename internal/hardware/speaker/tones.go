package speaker

import (
	"math"

	sharedtone "github.com/ygelfand/libcountertop/pkg/audio/tone"
)

type Note = sharedtone.Note

const toneLevel = sharedtone.Level

func (d *Driver) Chime(notes []Note) {
	if d == nil || len(notes) == 0 {
		return
	}
	d.Interject(func(s *Speaker) { s.Chime(toneLevel, notes...) })
}

func (s *Speaker) Beep(freq float64, ms int, level float64) {
	s.Chime(level, Note{Freq: freq, Ms: ms})
}

func (s *Speaker) Chime(level float64, notes ...Note) {
	var out []int16
	for _, n := range notes {
		out = append(out, tone(n, level)...)
	}
	s.Overlay(out)
}

func tone(n Note, level float64) []int16 {
	frames := Rate * n.Ms / 1000
	out := make([]int16, frames*Channels)

	for i := range frames {
		if n.Freq == 0 {
			continue
		}

		env := math.Min(1, math.Min(float64(i), float64(frames-i))/float64(Rate/200))
		v := int16(level * env * math.MaxInt16 * math.Sin(2*math.Pi*n.Freq*float64(i)/Rate))

		out[i*Channels] = v
		out[i*Channels+1] = v
	}
	return out
}
