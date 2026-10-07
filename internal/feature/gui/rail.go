package gui

import (
	gogui "github.com/go-gui-org/go-gui/gui"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/drawer"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/libcountertop/pkg/display/widgets"
)

func (a *App) railScreen(v shell.View) *Screen {
	return &Screen{View: v, Build: func(w *gogui.Window) gogui.View { return a.rail(w, v) }}
}
func (a *App) rail(w *gogui.Window, v shell.View) gogui.View {
	var entries []widgets.RailEntry
	for _, e := range drawer.Get().Entries() {
		glyph := ""
		if e.Glyph != nil {
			glyph = e.Glyph()
		}
		entries = append(entries, widgets.RailEntry{Label: e.Label(), Glyph: glyph, Open: e.Open})
	}
	return presentation.Toolkit.Rail(
		w,
		widgets.RailOptions{
			Entries: entries,
			Edge:    config.Get().Screen.Drawer,
			Close:   func() { shell.Get().Remove(v) },
		},
	)
}
