package gui

import (
	gogui "github.com/go-gui-org/go-gui/gui"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/libcountertop/pkg/display/widgets"
	"github.com/ygelfand/libcountertop/pkg/say"
)

func (a *App) keyboard(w *gogui.Window) gogui.View {
	a.mu.Lock()
	r := a.r
	a.mu.Unlock()
	if r == nil {
		return nil
	}
	at, view := presentation.Editor.Keyboard(w, widgets.KeyboardOptions{
		Size: widgets.KeyboardSize(
			config.Get().Screen.Keyboard,
		),
		Palette: presentation.Palette(),
		Kit:     presentation.Kit(),
		Cancel: say.T(
			"keyboard.cancel",
		),
		Save:  say.T("keyboard.save"),
		Type:  r.Type,
		Press: r.Press,
	})
	return placed(at, view)
}
