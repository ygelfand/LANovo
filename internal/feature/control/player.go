package control

import (
	sharedsource "github.com/ygelfand/libcountertop/pkg/media/source"

	"github.com/ygelfand/LANovo/internal/feature/media"
)

func player(a []string) (string, error) {
	p := media.Get()
	name, external := p.Sourced()
	return sharedsource.Report(p.Now(), name, external, a)
}
