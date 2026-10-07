package media

import (
	"sync"

	gogui "github.com/go-gui-org/go-gui/gui"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/drawer"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	sharedcard "github.com/ygelfand/libcountertop/pkg/display/mediacard"
	"github.com/ygelfand/libcountertop/pkg/say"
)

type screen = sharedcard.Screen

var cardOnce sync.Once
var theCard *sharedcard.Card

func card() *sharedcard.Card {
	cardOnce.Do(func() {
		theCard = sharedcard.New(sharedcard.Dependencies{Player: Get(), Idle: config.IdleSection, Shell: shell.Get()})
	})
	return theCard
}
func Page() shell.View { return card().Page() }
func Showing() bool    { return shell.Get().Top() == Page() }
func onRail() {
	drawer.Get().
		Add(drawer.Entry{Name: func() string { return say.T("rail.media") }, Order: drawer.OrderPlayer, Glyph: func() string { return gogui.IconMusic }, Open: func() {
			if now := Get().Now(); now.Playing || now.Paused {
				card().Open()
				return
			}
			shell.Get().Push(Page())
		}})
}

// transport is whoever the buttons reach: whoever holds the card, or Home Assistant before anything
// has played.
func (p *Player) Transport() Controller {
	if s := p.source(); s != nil {
		return s
	}
	return homeAssistant{}
}

// homeAssistant is a url played at this device through the media player entity: a peer of a group,
// a phone and a cast, claiming the card the same way.
//
// It answers only to stopping, since a url has no transport beyond ending it.
type homeAssistant struct{}

func (homeAssistant) Play()         { Get().stream.Unpause() }
func (homeAssistant) Pause()        { Get().stream.Pause() }
func (homeAssistant) Next()         {}
func (homeAssistant) Previous()     {}
func (homeAssistant) Stop()         { Get().Stop() }
func (homeAssistant) Now() Now      { return Get().queued() }
func (homeAssistant) Kind() Kind    { return FromQueue }
func (homeAssistant) Label() string { return "" }

func (p *Player) Holder() Source { return p.source() }

// ResetSession drops the media card's completed source session.
func (p *Player) ResetSession() { p.sourceOwner().ResetSession() }
