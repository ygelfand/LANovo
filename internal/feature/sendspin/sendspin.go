package sendspin

import (
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	sharedplayer "github.com/ygelfand/libcountertop/pkg/media/sendspin"
	"sync"
	"time"
)

type Player struct{ *sharedplayer.Player }

func (p *Player) Restore(c config.Config) { p.Player.Restore(c.Sendspin.Enabled) }

var once sync.Once
var shared *Player

func Get() *Player {
	once.Do(func() {
		shared = &Player{sharedplayer.New(sharedplayer.Options{Output: newOutput(), Arbitration: arbitration{speaker.Sound().Backgrounds()}, Identity: identity, Read: func() bool { return config.Get().Sendspin.Enabled }, Save: func(v bool) error { return config.Set().Sendspin().Enabled(v) }, Name: func() string { return config.Get().Device.Name }, DeviceID: component.DevicePlayback, Stepped: func(f func(time.Duration)) func() { return clock.Get().Stepped.Listen(f) }, Advertise: advertise, Media: func() sharedplayer.Media { return media.Get() }})}
	})
	return shared
}
func init() { component.Register(component.Device, Get, component.Order(26)) }
