package gui

import (
	gogui "github.com/go-gui-org/go-gui/gui"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/volume"
	sharedview "github.com/ygelfand/libcountertop/pkg/display/playerview"
	"github.com/ygelfand/libcountertop/pkg/say"
	"sync"
)

var playerViewOnce sync.Once
var playerView *sharedview.Renderer

func playerViews() *sharedview.Renderer {
	playerViewOnce.Do(func() {
		playerView = sharedview.New(sharedview.Options{HoldStill: holdStill, Now: media.Get().Now, Named: media.Get().Named, Transport: func() media.Controller { return media.Transport() }, Open: media.Get().Open, Heading: media.Heading, Volume: func() int { return volume.Get().Level(config.StreamMedia) }, SetVolume: func(v int) { volume.Get().Set(config.StreamMedia, v) }, Shell: shell.Get(), Toolkit: toolkit, Presses: interactions, Grip: grip, Palette: palette})
	})
	return playerView
}
func playerScreen(v shell.View) *Screen {
	return &Screen{Title: say.T("player.now"), View: v, Fixed: true, Build: playerViews().Body}
}
func (a *App) mini(w *gogui.Window) gogui.View { return playerViews().Mini(w) }
func playerBody(w *gogui.Window) gogui.View    { return playerViews().Body(w) }

var still = sharedview.Still
var squared = sharedview.Squared
var clockText = sharedview.ClockText

const priorityMini = sharedview.PriorityMini
