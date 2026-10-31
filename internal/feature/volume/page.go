package volume

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/display/idleview"
	sharedshell "github.com/ygelfand/libcountertop/pkg/display/shell"
	sharedview "github.com/ygelfand/libcountertop/pkg/display/volume"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/feedback"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/tessera"
)

var page = sync.OnceValue(func() *sharedshell.Page {
	return sharedview.NewPage(sharedview.PageOptions{
		Levels:    Get(),
		Streams:   config.Streams(),
		MainSteps: MainSteps,
		Chimes:    chimes{},
		Feedback:  feedback.Get().Feedback,
		Media:     Get().duck,
		Push:      shell.Get().Push,
	})
})

func Page() sharedshell.View { return page() }

type shows interface{ Shows(config.Stream) bool }

func showing(s config.Stream) bool {
	top := shell.Get().Top()
	if top == sharedshell.View(page()) {
		return true
	}
	if _, idle := top.(*idleview.View); idle && s == config.StreamMedia &&
		tessera.Firmware().SaverUp() {
		return true
	}
	v, ok := top.(shows)
	return ok && v.Shows(s)
}

type chimes struct{}

func (chimes) Chime() schema.Chime     { return config.Get().Feedback.Chime }
func (chimes) SetChime(c schema.Chime) { feedback.Get().SetChime(c) }
func (chimes) Preview(s schema.Stream) { feedback.Preview(s) }
