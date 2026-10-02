package settings

import (
	"log/slog"
	"strconv"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/feature/rtspd"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/lib/say"
	"github.com/ygelfand/LANovo/internal/setting"
	"github.com/ygelfand/LANovo/internal/ui/widget"
)

// shown is the groups this panel offers: the ones with something in them.
func shown() []setting.Group {
	var out []setting.Group
	for _, g := range livecam.Table().Groups() {
		if len(livecam.Table().In(g)) > 0 {
			out = append(out, g)
		}
	}
	return out
}

func set() livecam.Knobs { return livecam.Saved() }

func save(name, value string) {
	if err := livecam.Set(name, value); err != nil {
		slog.Error("the camera setting could not be saved", "setting", name, "err", err)
	}
}

const resetWindow = 3 * time.Second

var resetArmed time.Time

func cameraPage() *shell.Page {
	return camPage(&shell.Page{
		Title:   say.T("settings.camera"),
		Preview: camWant,
		Build: func() ([]widget.Row, []func(int)) {
			sh := set()

			var rows []widget.Row
			var taps []func(int)

			for _, g := range shown() {
				if in := livecam.Table().In(g); len(in) == 1 {
					row, tap := cameraRow(in[0], &sh)
					row.Label = livecam.Table().Title(g)
					rows = append(rows, row)
					taps = append(taps, tap)
					continue
				}
				rows = append(rows, widget.Row{
					Label: livecam.Table().Title(g),
					Kind:  widget.Chevron,
					Value: livecam.Table().Sums(g, &sh),
				})
				taps = append(taps, open(sectionPage(g)))
			}
			rows = append(rows, widget.Row{Label: say.T("camera.stream"), Hint: say.T("features.rtsp.hint"),
				Kind: widget.Toggle, On: config.Get().RTSP.Enabled})
			taps = append(taps, func(int) { rtspd.Get().SetEnabled(!config.Get().RTSP.Enabled) })
			label := say.T("camera.reset")
			if time.Since(resetArmed) < resetWindow {
				label = say.T("camera.reset.confirm")
			}
			rows = append(rows, widget.Row{Label: label})
			taps = append(taps, func(int) {
				if time.Since(resetArmed) >= resetWindow {
					resetArmed = time.Now()
					time.AfterFunc(resetWindow, shell.Get().Redraw)
					shell.Get().Redraw()
					return
				}
				resetArmed = time.Time{}
				if err := livecam.Reset(); err != nil {
					slog.Error("resetting the camera settings", "err", err)
				}
				shell.Get().Redraw()
			})
			return rows, taps
		},
	})
}

func watching(p *shell.Page) *shell.Page {
	p.Preview = camWant
	return camPage(p)
}

func sectionPage(g setting.Group) *shell.Page {
	return watching(&shell.Page{
		Title: livecam.Table().Title(g),
		Build: func() ([]widget.Row, []func(int)) {
			sh := set()

			var rows []widget.Row
			var taps []func(int)

			for _, s := range livecam.Table().In(g) {
				row, tap := cameraRow(s, &sh)
				rows = append(rows, row)
				taps = append(taps, tap)
			}
			return rows, taps
		},
	})
}

func cameraRow(s livecam.Knob, sh *livecam.Knobs) (widget.Row, func(int)) {
	row := widget.Row{Label: s.Title()}
	idle := s.Dim(sh)

	switch s.Kind {
	case setting.Toggle:
		row.Kind = widget.Toggle
		row.On = s.On(sh)
		return row, dimmed(&row, idle, func(int) {
			save(s.Name, setting.OnOff(!row.On))
		})

	case setting.Number:
		row.Kind = widget.Slider
		row.Level = s.Percent(s.Level(sh))
		row.Snap = func(level int) int { return s.Percent(s.Raw(level)) }

		// An absolute knob says its value. A doubling one says how far along it is: a gain of 128
		// sixteenths means nothing next to a bar, and half way does.
		if s.Scale == setting.Linear {
			row.Value = s.Read(sh)
			if s.Unit != "" {
				row.Value += " " + s.Unit
			}
		}
		return row, dimmed(&row, idle, func(level int) {
			save(s.Name, strconv.Itoa(s.Raw(level)))
		})

	case setting.Choice:
		row.Kind = widget.Chevron
		row.Value = s.Shows(sh)
		return row, dimmed(&row, idle, open(choicePage(s)))
	}

	row.Kind = widget.Plain
	row.Value = s.Read(sh)
	return row, nil
}

// dimmed fades a row that has nothing to do and takes its action away.
func dimmed(row *widget.Row, idle bool, act func(int)) func(int) {
	if !idle {
		return act
	}
	row.Dim = true
	return nil
}

func choicePage(s livecam.Knob) *shell.Page {
	return watching(&shell.Page{
		Title: s.Title(),
		Build: func() ([]widget.Row, []func(int)) {
			// Read afresh: the page stays up while options are picked, so a value captured when
			// it opened would leave the mark on whatever was chosen first.
			cfg := set()
			at := s.Read(&cfg)

			rows := make([]widget.Row, 0, len(s.Options))
			taps := make([]func(int), 0, len(s.Options))

			for _, o := range s.Options {
				rows = append(rows, widget.Row{
					Label:  s.Label(o),
					Kind:   widget.Plain,
					Chosen: o.Value == at,
				})
				taps = append(taps, func(int) {
					save(s.Name, o.Value)
				})
			}
			return rows, taps
		},
	})
}
