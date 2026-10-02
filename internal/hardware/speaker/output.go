package speaker

import (
	"math"
	"time"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/lib/alsa"
)

// output is the part of the playback path that differs by SoC.
type output interface {
	route() []setting
	stereo() bool
	power(out *alsa.Playback, on bool) error
	pipeline() time.Duration
	volume(m *alsa.Mixer, dB float64) (bool, error)
}

func outputFor(b board.Board) output {
	if b.SoC == board.MediaTek {
		return newMediatek(b)
	}
	return &qualcomm{}
}

func (s *Speaker) hw() output {
	s.hwOnce.Do(func() { s.hwOut = outputFor(board.Current()) })
	return s.hwOut
}

// decibels is a gain as attenuation, with silence as negative infinity.
func decibels(gain float32) float64 {
	if gain <= 0 {
		return math.Inf(-1)
	}
	return 20 * math.Log10(float64(gain))
}

func (s *Speaker) Pipeline() time.Duration { return s.hw().pipeline() }
