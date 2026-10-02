package volume

import (
	"sync"

	"github.com/ygelfand/LANovo/internal/lib/say"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/feedback"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/ui/widget"
)

// Page is the screen for setting every level, one slider each.
//
// A stream nobody has a use for still gets a row: four sliders is a screen someone reads once and
// then knows, where hiding the quiet ones would make it a different screen every time.
func Page() shell.View {
	pageOnce.Do(func() { page = build() })
	return page
}

var (
	pageOnce sync.Once
	page     *shell.Page
)

// shows is a view that already has a level on it. The card says what a level changed to, which is
// worth saying over a clock and not over the slider that changed it.
type shows interface{ Shows(config.Stream) bool }

func showing(s config.Stream) bool {
	top := shell.Get().Top()
	if top == Page() {
		return true
	}
	v, ok := top.(shows)
	return ok && v.Shows(s)
}

func build() *shell.Page {
	return &shell.Page{
		Title: say.T("settings.volume"),
		Build: func() ([]widget.Row, []func(int)) {
			streams := config.Streams()

			rows := make([]widget.Row, 0, len(streams))
			acts := make([]func(int), 0, len(streams))

			for _, s := range streams {
				level := Get().Level(s)
				rows = append(rows, widget.Row{
					Label: say.T("sound." + string(s)),
					Kind:  widget.Slider,
					Level: level,
				})
				acts = append(acts, set(s))
			}

			// Under the levels, because it is the same question asked once more: how much noise
			// should this thing make. None is here rather than in a corner of the settings,
			// which is where somebody looks when the device beeped at them at two in the morning.
			rows = append(rows, widget.Row{
				Label: say.T("volume.chime"),
				Kind:  widget.Chevron,
				Value: config.Get().Feedback.Chime.Label(),
			})
			acts = append(acts, func(int) { shell.Get().Push(chimePage()) })

			return rows, acts
		},
	}
}

func set(s config.Stream) func(int) {
	return func(level int) { Get().Set(s, level) }
}

// chimePage picks what the device sounds like when it acknowledges something, None included.
func chimePage() *shell.Page {
	return &shell.Page{
		Title: say.T("volume.chime"),
		Build: func() ([]widget.Row, []func(int)) {
			now := config.Get().Feedback.Chime

			var (
				rows []widget.Row
				acts []func(int)
			)
			for _, c := range config.Chimes() {
				rows = append(rows, widget.Row{Label: c.Label(), Chosen: c == now})
				acts = append(acts, useChime(c))
			}
			return rows, acts
		},
	}
}

// useChime saves the choice and plays it, so picking one is how you hear it. None plays nothing,
// which is the answer to whether it worked.
func useChime(c config.Chime) func(int) {
	return func(int) {
		feedback.Get().SetChime(c)
		feedback.Volume()
	}
}
