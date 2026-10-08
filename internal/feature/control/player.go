package control

import (
	"github.com/ygelfand/libcountertop/pkg/media/nowplaying"

	"github.com/ygelfand/LANovo/internal/feature/media"
)

func player(a []string) (string, error) {
	p := media.Get()
	name, external := p.Sourced()
	return nowplaying.Report(p.Now(), name, external, a)
}
