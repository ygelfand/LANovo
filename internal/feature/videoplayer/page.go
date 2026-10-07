package videoplayer

import (
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	sharedvolume "github.com/ygelfand/libcountertop/pkg/audio/volume"
	sharedpage "github.com/ygelfand/libcountertop/pkg/display/videopage"
)

type Page = sharedpage.Page
type Look = sharedpage.Look

var Watched = sharedpage.Watched

func NewPage(c Controls) *Page {
	return sharedpage.New(c, sharedpage.Dependencies{Shell: shell.Get(), Volume: sharedvolume.For(volume.Get(), config.StreamMedia)})
}
