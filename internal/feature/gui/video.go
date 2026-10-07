package gui

import (
	gogui "github.com/go-gui-org/go-gui/gui"
	"github.com/ygelfand/LANovo/internal/feature/videoplayer"
)

func (a *App) videoScreen(p *videoplayer.Page) *Screen {
	return &Screen{View: p, Fixed: true, Build: func(w *gogui.Window) gogui.View { return playerViews().Video(w, p) }}
}
func (a *App) video(w *gogui.Window, p *videoplayer.Page) gogui.View {
	return playerViews().Video(w, p)
}
