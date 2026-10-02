package assistant

import (
	"math"
	"time"

	"github.com/ygelfand/LANovo/internal/feature/voice"
)

// Showing is a turn as the panel draws it: what the conversation says is happening, and how far
// through the arrival and the answer it is.
type Showing struct {
	voice.Showing

	// Down is how far the panel has slid, nought at the top edge and one fully out. A turn ending
	// runs the same number backwards.
	Down float64

	// Level is how loud the room is, nought to one, which is what the wave is drawn from.
	Level float64

	// At is the moment the frame is for. The animation moves with it.
	At time.Time

	// Reveal is how much of the reply has been spoken, nought to one.
	Reveal float64
}

// Revealed is how much of the reply has been spoken, cut on a rune boundary so a multi-byte
// character is never drawn as half of itself.
//
// By character rather than by word: a word appearing whole arrives ahead of the voice saying it,
// and keeping pace is the only reason to reveal it rather than print it.
func Revealed(text string, part float64) string {
	switch runes := []rune(text); {
	case part <= 0:
		return ""
	case part >= 1:
		return text
	default:
		return string(runes[:int(float64(len(runes))*part)])
	}
}

// How the wave is drawn.
const (
	// waveBars is enough to read as a wave and few enough that each stays wide enough to see from
	// across a room.
	waveBars = 21

	// waveFloor is what the resting wave has to itself, as a fraction of the band. The room only
	// ever adds to this, so a silent one still has a wave rather than a row of stubs.
	waveFloor = 0.30

	// waveRest is how much of the floor travels. The rest of it is the height the bars never drop
	// below, which is what stops the wave flickering out of existence between crests.
	waveRest = 0.65

	// waveHz is how fast the travelling part moves, in cycles a second.
	waveHz = 0.8
)

// wave is the room as a row of bars: a shape that breathes with the level, with a travelling
// component so it still moves while nobody is saying anything.
func Bars(show Showing) []float64 {
	t := seconds(show.At)
	level := min(max(show.Level, 0), 1)
	bars := make([]float64, waveBars)
	for i := range bars {
		at := float64(i) / float64(waveBars-1)
		bell := math.Sin(at * math.Pi)
		travel := 0.5 + 0.5*math.Sin(2*math.Pi*(waveHz*t-at*1.5))
		rest := waveFloor * (1 - waveRest + waveRest*travel)
		bars[i] = rest + (1-waveFloor)*level*bell*travel
	}
	return bars
}

const (
	thinkDots  = 3
	thinkCycle = 1.1
)

func Dots(show Showing) []float64 {
	t := seconds(show.At)
	lifts := make([]float64, thinkDots)
	for i := range lifts {
		lifts[i] = max(math.Sin(2*math.Pi*(t/thinkCycle-float64(i)/float64(thinkDots)/2)), 0)
	}
	return lifts
}

func seconds(at time.Time) float64 {
	return float64(at.UnixNano()) / float64(time.Second)
}
