// Package reveal binds product artwork to the shared boot reveal.
package reveal

import (
	_ "embed"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	shared "github.com/ygelfand/libcountertop/pkg/display/boot"
)

//go:embed reveal.glsl
var revealShader string

const LogoShare = shared.LogoShare

type Moment = shared.Moment
type Reveal = shared.Reveal

var Split = shared.Split

func New(palette theme.Theme) *Reveal {
	src := ui.Logo()
	if theme.Dark(palette.Background) {
		src = ui.Night()
	}
	return shared.NewReveal(palette, revealShader, src, 1000, 666)
}
