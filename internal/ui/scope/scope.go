package scope

import (
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

type Kind string

const (
	Wave        Kind = "wave"
	VU          Kind = "vu"
	Spectrum    Kind = "spectrum"
	Spectrogram Kind = "spectrogram"
	KindCount        = 4
)

const Default = Wave

type Scope interface {
	Draw(s ui.Surface, in ui.Rect, f Frame, ink theme.Color, palette theme.Theme)
}

var registered = map[Kind]Scope{}

func register(k Kind, s Scope) { registered[k] = s }

func Of(k Kind) Scope {
	if s, ok := registered[k]; ok {
		return s
	}
	return registered[Default]
}

func Kinds() []Kind { return []Kind{Wave, VU, Spectrum, Spectrogram} }

func (k Kind) Label() string {
	switch k {
	case VU:
		return "VU meter"
	case Spectrum:
		return "Spectrum"
	case Spectrogram:
		return "Spectrogram"
	}
	return "Waveform"
}
