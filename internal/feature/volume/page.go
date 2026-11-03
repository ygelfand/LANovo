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

func showing(s config.Stream) bool {
	top := shell.Get().Top()
	if top == sharedshell.View(page()) {
		return true
	}
	_, idle := top.(*idleview.View)
	return idle && s == config.StreamMedia && tessera.Firmware().SaverUp()
}

type chimes struct{}

func (chimes) Chime() schema.Chime     { return config.Get().Feedback.Chime }
func (chimes) SetChime(c schema.Chime) { feedback.Get().SetChime(c) }
func (chimes) Preview(s schema.Stream) { feedback.Preview(s) }
