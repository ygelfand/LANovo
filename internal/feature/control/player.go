package control

import (
	"github.com/ygelfand/LANovo/internal/feature/media"
	sharedsource "github.com/ygelfand/libcountertop/pkg/media/source"
)

func player(a []string) (string, error) {
	p := media.Get()
	name, external := p.Sourced()
	return sharedsource.Report(p.Now(), name, external, a)
}
