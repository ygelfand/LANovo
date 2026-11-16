package volume

import (
	"log/slog"
	"sync"

	"github.com/ygelfand/libcountertop/pkg/display/idleview"
	"github.com/ygelfand/libcountertop/pkg/display/menu"
	sharedshell "github.com/ygelfand/libcountertop/pkg/display/shell"
	sharedview "github.com/ygelfand/libcountertop/pkg/display/volume"
	"github.com/ygelfand/libcountertop/pkg/say"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/feedback"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/tessera"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

var page = sync.OnceValue(func() *sharedshell.Page {
	return sharedview.NewPage(sharedview.PageOptions{
		Levels:    Get(),
		Streams:   config.Streams(),
		MainSteps: MainSteps,
		Chimes:    chimes{},
		Feedback:  feedback.Get().Feedback,
		Media:     Get().duck,
		Outputs:   speakerRows,
		Push:      shell.Get().Push,
	})
})

func speakerRows() ([]menu.Row, []func(int)) {
	available, on := speaker.Get().EQ()
	if !available {
		return nil, nil
	}
	return []menu.Row{{Label: say.T("speaker.setting.eq"), Kind: menu.Toggle, On: on}},
		[]func(int){func(int) {
			if err := speaker.Get().SetEQ(!on); err != nil {
				slog.Warn("the speaker EQ could not be saved", "err", err)
			}
		}}
}

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
