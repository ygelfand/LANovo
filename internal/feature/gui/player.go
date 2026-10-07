package gui

import (
	"sync"

	gogui "github.com/go-gui-org/go-gui/gui"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	sharedvolume "github.com/ygelfand/libcountertop/pkg/audio/volume"
	sharedview "github.com/ygelfand/libcountertop/pkg/display/playerview"
	"github.com/ygelfand/libcountertop/pkg/say"
)

var playerViewOnce sync.Once
var playerView *sharedview.Renderer

func playerViews() *sharedview.Renderer {
	playerViewOnce.Do(func() {
		playerView = sharedview.New(sharedview.Dependencies{Player: media.Get(), Volume: sharedvolume.For(volume.Get(), config.StreamMedia), Shell: shell.Get(), UI: presentation})
	})
	return playerView
}
func playerScreen(v shell.View) *Screen {
	return &Screen{Title: say.T("player.now"), View: v, Fixed: true, Build: playerViews().Body}
}
func (a *App) mini(w *gogui.Window) gogui.View { return playerViews().Mini(w) }
