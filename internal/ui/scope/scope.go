// Package scope is the ways sound can be drawn.
//
// The same shape as the clock faces, and for the same reason: something that draws takes the box it
// is given, the theme and a color, and paints inside it. A scope is not a place — the clock can
// carry one along its foot, the player one behind the artwork, a voice turn one under what was
// heard — so placement is the host's business and never the scope's.
//
// One file per scope, registering itself from init. Nothing outside asks for one by its type: a
// name picks it and Of hands it back.
//
// What a scope draws from is a Frame: a period of audio already reduced to what drawing needs.
// Reducing it once here rather than in each scope is what lets a host give the same frame to two of
// them, and what keeps the samples out of the drawing code.
package scope

import (
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Kind names a scope.
type Kind string

const (
	Wave        Kind = "wave"
	VU          Kind = "vu"
	Spectrum    Kind = "spectrum"
	Spectrogram Kind = "spectrogram"
	KindCount        = 4
)

// Default is what an unknown name falls back to.
const Default = Wave

// Scope draws a frame of audio inside the box it is given, in the color it is given.
//
// Inside the box is the contract. A scope goes wherever a host puts it, often over something else,
// and one that painted outside would overwrite whatever that was.
type Scope interface {
	Draw(s ui.Surface, in ui.Rect, f Frame, ink theme.Color, palette theme.Theme)
}

// registered is every scope there is.
var registered = map[Kind]Scope{}

func register(k Kind, s Scope) { registered[k] = s }

// Of is the scope a name picks, or the default for one this build does not have.
func Of(k Kind) Scope {
	if s, ok := registered[k]; ok {
		return s
	}
	return registered[Default]
}

// Kinds is every scope there is, in a stable order, for anything offering a choice of them.
func Kinds() []Kind { return []Kind{Wave, VU, Spectrum, Spectrogram} }

// Label is the name as somebody choosing one would read it.
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
