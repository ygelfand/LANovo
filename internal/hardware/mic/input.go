package mic

import (
	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/lib/alsa"
)

const (
	MinGain     = 0
	MaxGain     = 124
	DefaultGain = 84
)

type input interface {
	device() int
	bits() int
	open(m *alsa.Mixer) error
	gain(m *alsa.Mixer, gain int) error
}

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
