package speaker

import (
	"math"

	sharedtone "github.com/ygelfand/libcountertop/pkg/audio/tone"
)

// Note is one tone in a chime. A zero frequency is a rest.
type Note = sharedtone.Note

// The sounds the device makes about itself, as opposed to anything it was asked to play.
//
// Direction carries the meaning — rising for on, falling for off — and a hold is two notes where a
// press is one, so they are told apart without looking.
const toneLevel = sharedtone.Level

// Chime plays a tone alongside whatever is playing rather than instead of it: pressing volume during
// a reply should beep and leave the reply alone.
func (d *Driver) Chime(notes []Note) {
	if d == nil || len(notes) == 0 {
		return
	}
	d.Interject(func(s *Speaker) { s.Chime(toneLevel, notes...) })
}

// Beep queues a tone.
func (s *Speaker) Beep(freq float64, ms int, level float64) {
	s.Chime(level, Note{Freq: freq, Ms: ms})
}

// Chime sounds notes back to back, each shaped by the same short envelope so the joins do not click.
//
// A tone is feedback, so it mixes into whatever is queued rather than waiting behind it: queued
// audio is a reply's cushion or, when the reply came as a file, all of it, and a beep that waits is
// a beep that arrives after the answer.
func (s *Speaker) Chime(level float64, notes ...Note) {
	var out []int16
	for _, n := range notes {
		out = append(out, tone(n, level)...)
	}
	s.Overlay(out)
}

// tone is one note, ramped in and out so the ends do not click.
func tone(n Note, level float64) []int16 {
	frames := Rate * n.Ms / 1000
	out := make([]int16, frames*Channels)

	for i := range frames {
		if n.Freq == 0 {
			continue
		}

		// The ramp is a two-hundredth of a second at each end, whichever end is nearer.
		env := math.Min(1, math.Min(float64(i), float64(frames-i))/float64(Rate/200))
		v := int16(level * env * math.MaxInt16 * math.Sin(2*math.Pi*n.Freq*float64(i)/Rate))

		out[i*Channels] = v
		out[i*Channels+1] = v
	}
	return out
}
