package sendspin

import (
	"sync"

	sharedplayer "github.com/ygelfand/libcountertop/pkg/media/sendspin"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

type componentPlayer struct{ *sharedplayer.Player }

func (p *componentPlayer) Restore(c config.Config) { p.Player.Restore(c.Sendspin.Enabled) }

var get = sync.OnceValue(func() *sharedplayer.Player {
	return sharedplayer.New(sharedplayer.Dependencies{
		Output:      newOutput(),
		Arbitration: speaker.Sound().Backgrounds(),
		Device:      identity{},
		Settings:    config.SendspinSection,
		DeviceID:    component.DevicePlayback,
		Clock:       clock.Get(),
		Media:       media.Get(),
	})
})

func Get() *sharedplayer.Player { return get() }
func init() {
	component.Register(
		sharedcomponent.Device,
		func() *componentPlayer { return &componentPlayer{Get()} },
		sharedcomponent.Order(26),
	)
}
