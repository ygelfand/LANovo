package videoplayer

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	sharedpage "github.com/ygelfand/libcountertop/pkg/display/videopage"
)

type Page = sharedpage.Page
type Look = sharedpage.Look

var Watched = sharedpage.Watched

func NewPage(c Controls) *Page {
	return sharedpage.New(c, sharedpage.Options{Shell: shell.Get(), Level: func() int { return volume.Get().Level(config.StreamMedia) }, VolumeChanged: func(f func()) func() {
		return volume.Get().Changed.Listen(func(c volume.Change) {
			if c.Stream == config.StreamMedia {
				f()
			}
		})
	}})
}
