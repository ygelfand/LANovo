package speaker

import (
	"math"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
)

// Note is one tone in a chime. A zero frequency is a rest.
type Note struct {
	Freq float64
	Ms   int
}

// The sounds the device makes about itself, as opposed to anything it was asked to play.
//
// Direction carries the meaning — rising for on, falling for off — and a hold is two notes where a
// press is one, so they are told apart without looking.
const toneLevel = 0.3

var (
	ToneVolume = []Note{{Freq: 880, Ms: 80}}

	ToneMute     = []Note{{Freq: 880, Ms: 70}, {Freq: 587, Ms: 110}}
	ToneUnmute   = []Note{{Freq: 587, Ms: 70}, {Freq: 880, Ms: 110}}
	ToneMuteHold = []Note{{Freq: 587, Ms: 70}, {Freq: 440, Ms: 130}}

	// ToneTrouble falls twice and ends low, which no acknowledgement does. A request that cannot be
	// served has to sound different from one that was, or a failure is indistinguishable from the
	// device having ignored the person entirely.
	ToneTrouble = []Note{{Freq: 622, Ms: 90}, {Freq: 466, Ms: 90}, {Freq: 349, Ms: 180}}

	// ToneCancel is one short falling pair: the request was dropped, which is neither a failure nor
	// an answer.
	ToneCancel = []Note{{Freq: 698, Ms: 60}, {Freq: 466, Ms: 90}}

	// ToneTimer is a timer that has finished. Three of the same note, because it repeats until
	// somebody stops it and a melody wears out faster than a beep does.
	ToneTimer = []Note{
		{Freq: 880, Ms: 160}, {Ms: 130},
		{Freq: 880, Ms: 160}, {Ms: 130},
		{Freq: 880, Ms: 160},
	}
)

// Length is how long notes take to sound, rests included.
func Length(notes []Note) time.Duration {
	var ms int
	for _, n := range notes {
		ms += n.Ms
	}
	return time.Duration(ms) * time.Millisecond
}

// chimes is what an acknowledgement can sound like. Told apart by shape rather than pitch, so two
// devices in earshot set to different ones are distinguishable without knowing which is which.
//
// None maps to nothing, and Chime already declines to play an empty tone, so silence needs no
// special case anywhere downstream.
var chimes = map[config.Chime][]Note{
	config.ChimeNone:  nil,
	config.ChimeChirp: ToneVolume,
	config.ChimeDing:  {{Freq: 1319, Ms: 160}},
	config.ChimeRise:  {{Freq: 659, Ms: 55}, {Freq: 880, Ms: 55}, {Freq: 1319, Ms: 110}},
}

// ChimeTone is what an acknowledgement set to c sounds like.
func ChimeTone(c config.Chime) []Note { return chimes[c] }

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
