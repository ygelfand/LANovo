package wakeword

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/assistant/wakesettings"
	"github.com/ygelfand/libcountertop/pkg/hook"
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/tones"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(20))
}

const Slots = component.Assistants

var Requested hook.Hook[int]

type WakeWord struct{ *wakesettings.Settings }

var (
	once   sync.Once
	shared *WakeWord
)

func Get() *WakeWord {
	once.Do(func() {
		shared = &WakeWord{wakesettings.New(wakesettings.Options{
			Slots:    Slots,
			DeviceID: component.AssistantDevice,
			Write:    func(slot int) schema.WakeWriter { return config.Set().Wake(slot) },
			Wake:     Requested.Emit,
			Tones:    tones.Get().Choices,
		})}
		tones.Get().Changed.Listen(func(struct{}) {
			shared.Refresh(config.Get().Wake.Slots(Slots))
		})
	})
	return shared
}

func (w *WakeWord) Restore(c config.Config) { w.Settings.Restore(c.Wake.Slots(Slots)) }

func Threshold(slot int) float64 { return config.Get().Wake.Slot(slot).Threshold }
