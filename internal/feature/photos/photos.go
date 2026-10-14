package photos

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/media/photos"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/layout"
)

type componentPhotos struct{ *photos.Library }

func (p componentPhotos) Restore(c config.Config) { p.Library.Restore(c.Photos) }

var get = sync.OnceValue(func() *photos.Library {
	l := photos.New(photos.Options{
		Settings: config.PhotosSection,
		Dir:      layout.PhotoDir,
		DeviceID: component.DeviceScreen,
	})
	component.Settings.Add(l.Settings())
	return l
})

func Get() *photos.Library { return get() }

func init() {
	component.Register(
		sharedcomponent.Network,
		func() componentPhotos { return componentPhotos{Get()} },
		sharedcomponent.Order(24),
	)
}
