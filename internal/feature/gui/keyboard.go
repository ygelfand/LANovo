package gui

import (
	gogui "github.com/go-gui-org/go-gui/gui"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/libcountertop/pkg/display/widgets"
	"github.com/ygelfand/libcountertop/pkg/say"
)

var editor = widgets.NewEditor()

func textField(cfg gogui.InputCfg, save func()) gogui.View {
	return editor.Field(controls(), cfg, save)
}
func typing(w *gogui.Window) bool { return editor.Typing(w) }

func (a *App) keyboard(w *gogui.Window) gogui.View {
	a.mu.Lock()
	r := a.r
	a.mu.Unlock()
	if r == nil {
		return nil
	}
	at, view := editor.Keyboard(w, widgets.KeyboardOptions{
		Size: widgets.KeyboardSize(config.Get().Screen.Keyboard), Palette: palette(), Kit: controls(),
		Cancel: say.T("keyboard.cancel"), Save: say.T("keyboard.save"), Type: r.Type, Press: r.Press,
	})
	return placed(at, view)
}
