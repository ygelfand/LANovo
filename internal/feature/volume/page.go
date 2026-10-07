package volume

import (
	"sync"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/feedback"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	sharedview "github.com/ygelfand/libcountertop/pkg/display/volume"
)

var page = sync.OnceValue(func() *sharedview.Page { return sharedview.NewPage(Get(), config.Streams(), chimes{}, shell.Get()) })

func Page() shell.View { return page() }

type shows interface{ Shows(config.Stream) bool }

func showing(s config.Stream) bool { v, ok := shell.Get().Top().(shows); return ok && v.Shows(s) }

type chimes struct{}

func (chimes) Chime() config.Chime     { return config.Get().Feedback.Chime }
func (chimes) SetChime(c config.Chime) { feedback.Get().SetChime(c) }
func (chimes) Preview()                { feedback.Volume() }
