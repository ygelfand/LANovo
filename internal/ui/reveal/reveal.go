package reveal

import (
	_ "embed"

	shared "github.com/ygelfand/libcountertop/pkg/display/boot"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
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
