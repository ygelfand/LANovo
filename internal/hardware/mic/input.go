package mic

import (
	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/lib/alsa"
)

// The gain setting's range; DefaultGain is where each board's stock firmware leaves its microphones.
const (
	MinGain     = 0
	MaxGain     = 124
	DefaultGain = 84
)

// input is the part of the capture path that differs by SoC.
type input interface {
	device() int
	bits() int
	open(m *alsa.Mixer) error
	gain(m *alsa.Mixer, gain int) error
}

// setting is one mixer control and what to put in it: a number, or the name of a choice for an
// enumerated one.
type setting struct {
	name   string
	value  uint32
	choice string
}

func inputFor(b board.Board) input {
	if b.SoC == board.MediaTek {
		return &mediatek{chip: b.MicADC}
	}
	return qualcomm{}
}

func (m *Mics) hw() input {
	m.hwOnce.Do(func() { m.hwIn = inputFor(board.Current()) })
	return m.hwIn
}
