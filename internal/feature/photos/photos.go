package photos

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/media/photos"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/product"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

type componentPhotos struct{ *photos.Library }

func (p componentPhotos) Restore(c config.Config) { p.Library.Restore(c.Photos) }

var get = sync.OnceValue(func() *photos.Library {
	return photos.Build(product.Get(), config.PhotosSection, func() int {
		w, h := display.Get().Native()
		return max(w, h)
	})
})

func Get() *photos.Library { return get() }

func init() {
	component.Register(
		sharedcomponent.Network,
		func() componentPhotos { return componentPhotos{Get()} },
		sharedcomponent.Order(24),
	)
}
