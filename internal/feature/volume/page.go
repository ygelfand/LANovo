package volume

import (
	"sync"

	sharedshell "github.com/ygelfand/libcountertop/pkg/display/shell"
	sharedview "github.com/ygelfand/libcountertop/pkg/display/volume"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/feedback"
	"github.com/ygelfand/LANovo/internal/feature/shell"
)

var page = sync.OnceValue(
	func() *sharedview.Page { return sharedview.NewPage(Get(), config.Streams(), chimes{}, shell.Get()) },
)

func Page() sharedshell.View { return page() }

type shows interface{ Shows(config.Stream) bool }

func showing(s config.Stream) bool { v, ok := shell.Get().Top().(shows); return ok && v.Shows(s) }

type chimes struct{}

func (chimes) Chime() schema.Chime     { return config.Get().Feedback.Chime }
func (chimes) SetChime(c schema.Chime) { feedback.Get().SetChime(c) }
func (chimes) Preview()                { feedback.Volume() }
