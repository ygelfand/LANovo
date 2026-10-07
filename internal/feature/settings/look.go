package settings

import (
	"log/slog"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/ui/visual"
	"github.com/ygelfand/LANovo/internal/ui/widget"
	"github.com/ygelfand/libcountertop/pkg/say"
)

func lookPage(slot int) *shell.Page {
	return &shell.Page{
		Title: say.T("look.title"),
		Build: func() ([]widget.Row, []func(int)) {
			look := config.Get().Wake.Slot(slot).Look
			rows := []widget.Row{{Label: say.T("look.place"), Kind: widget.Chevron, Value: look.Place.Label()}}
			taps := []func(int){open(placePage(slot))}
			for _, st := range config.Stages() {
				rows = append(rows, widget.Row{Label: st.Label(), Kind: widget.Chevron, Value: lookName(look.Kind(st))})
				taps = append(taps, open(stagePage(slot, st)))
			}
			return rows, taps
		},
	}
}

func lookName(kind string) string {
	if kind == "" {
		return say.T("look.default")
	}
	return visual.Kind(kind).Label()
}

func placePage(slot int) *shell.Page {
	return &shell.Page{
		Title: say.T("look.place"),
		Build: func() ([]widget.Row, []func(int)) {
			now := config.Get().Wake.Slot(slot).Look.Place
			var rows []widget.Row
			var taps []func(int)
			for _, p := range config.LookPlaces() {
				rows = append(rows, widget.Row{Label: p.Label(), Chosen: p == now})
				taps = append(taps, func(int) {
					if err := config.Set().Wake(slot).Place(p); err != nil {
						slog.Error("saving the assistant visual placement failed", "slot", slot+1, "err", err)
					}
				})
			}
			return rows, taps
		},
	}
}

func stagePage(slot int, st config.Stage) *shell.Page {
	use := func(kind string) {
		if err := config.Set().Wake(slot).Visual(st, kind); err != nil {
			slog.Error("saving the assistant visual failed", "slot", slot+1, "stage", st, "err", err)
		}
		shell.Get().Pop()
	}
	return visualPicker(st.Label(), say.T("look.default"),
		func() { use("") },
		func() string { return config.Get().Wake.Slot(slot).Look.Kind(st) },
		func(k visual.Kind) { use(string(k)) })
}
