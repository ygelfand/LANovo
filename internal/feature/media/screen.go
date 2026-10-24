package media

import (
	"sync"

	sharedcard "github.com/ygelfand/libcountertop/pkg/display/mediacard"
	sharedshell "github.com/ygelfand/libcountertop/pkg/display/shell"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/apps"
	"github.com/ygelfand/LANovo/internal/feature/shell"
)

var cardOnce sync.Once
var theCard *sharedcard.Card

func card() *sharedcard.Card {
	cardOnce.Do(func() {
		theCard = sharedcard.New(
			sharedcard.Dependencies{Player: Get(), Idle: config.IdleSection, Shell: shell.Get()},
		)
	})
	return theCard
}
func Page() sharedshell.View { return card().Page() }
func Showing() bool          { return shell.Get().Top() == Page() }
func onRail()                { apps.Get().Add(card().App()) }

func init() {
	component.Register(
		sharedcomponent.Device,
		func() *sharedcard.Card { c := card(); onRail(); Get().Begun.Listen(c.Began); return c },
		sharedcomponent.Order(39),
	)
}
func (p *Player) Open() { card().Open() }
