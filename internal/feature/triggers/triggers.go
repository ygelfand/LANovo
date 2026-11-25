package triggers

import (
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/say"
	setting "github.com/ygelfand/libcountertop/pkg/settings"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
	"github.com/ygelfand/libcountertop/pkg/voice"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/actions"
	"github.com/ygelfand/LANovo/internal/feature/commands"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
)

func init() { component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(42)) }

type Triggers struct {
	knobs *setting.Controls[schema.Voice]
}

var get = sync.OnceValue(func() *Triggers {
	say.UseVoice(config.VoiceSection.Read().Language)
	t := &Triggers{knobs: &setting.Controls[schema.Voice]{
		Table: voice.NewTable(
			actions.Get().Names(),
			Commands(),
			voice.LabelChoices(homeassistant.Get()),
		),
		Device: component.DeviceVoice,
		Read:   config.VoiceSection.Read,
		Save:   save,
	}}
	component.Settings.Add(t.knobs)
	return t
})

func Get() *Triggers { return get() }

func Commands() []string {
	out := []string{"radar"}
	if board.Current().CameraWidth > 0 {
		out = append(out, "mirror")
	}
	return out
}

func (t *Triggers) Name() string { return "triggers" }

func (t *Triggers) Entities() []esphome.Entity { return t.knobs.Entities() }

func (t *Triggers) Restore(cfg config.Config) { t.knobs.PublishFrom(cfg.Voice) }

func (t *Triggers) Controls() *setting.Controls[schema.Voice] { return t.knobs }

func save(s setting.Setting[schema.Voice], v string) error {
	err := setting.Store(config.VoiceSection, s, v)
	say.UseVoice(config.VoiceSection.Read().Language)
	commands.Get().Apply()
	actions.Get().Declare()
	return err
}
