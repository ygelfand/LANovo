package doorbell

import (
	"sync"

	"github.com/ygelfand/LANovo/internal/feature/dashboard"

	esphome "github.com/ygelfand/go-esphome-device"

	shared "github.com/ygelfand/libcountertop/pkg/camera/doorbell"
	"github.com/ygelfand/libcountertop/pkg/display/doorbellview"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/alerts"
	"github.com/ygelfand/LANovo/internal/feature/cameras"
	"github.com/ygelfand/LANovo/internal/feature/idle"
	"github.com/ygelfand/LANovo/internal/feature/message"
	"github.com/ygelfand/LANovo/internal/feature/product"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/mic"
	"github.com/ygelfand/LANovo/internal/hardware/speaker"
)

func init() {
	component.Register(sharedcomponent.Network, func() *Doorbell { return &Doorbell{Get()} })
}

var Get = sync.OnceValue(func() *shared.Doorbell {
	return shared.Build(product.Get(), config.DoorbellSection, alerts.Get().Tones)
})

var View = sync.OnceValue(func() *doorbellview.View {
	v := doorbellview.New(doorbellview.Options{
		Shell:    shell.Get(),
		Bell:     Get(),
		Display:  display.Get(),
		Speaker:  sound{},
		Mic:      func() (<-chan []int16, func()) { return mic.Get().Listen("doorbell") },
		Messages: message.Get(),
		Chime:    func(c schema.Chime) { alerts.Get().RingAlarm(c, 1) },
		Cameras:  cameras.Get(),
		Settings: config.DoorbellSection,
		Clock24:  func() bool { return config.Get().Screen.Hours == schema.TwentyFourHour },
		Wake:     func() { idle.Get().Wake() },
	})
	dashboard.Tabs().Add(30, v.DashboardTabs)
	return v
})

type Doorbell struct{ *shared.Doorbell }

func (d *Doorbell) Entities() []esphome.Entity { return d.Doorbell.Entities() }

func (d *Doorbell) Restore(cfg config.Config) { d.Doorbell.Restore(cfg.Doorbell) }

type sound struct{}

func (sound) Play(pcm []int16) { speaker.Get().PlayStream(config.StreamVoice, pcm) }

func (sound) Queued() int { return speaker.Get().Queued() }
