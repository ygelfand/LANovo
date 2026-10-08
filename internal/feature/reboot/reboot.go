package reboot

import (
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/runtime/safe"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/component"
)

func init() {
	component.Register(component.Device, Get, component.Order(90))
}

type Reboot struct{ button *esphome.Button }

var (
	once   sync.Once
	shared *Reboot
)

func Get() *Reboot {
	once.Do(func() {
		shared = &Reboot{}
		shared.build()
	})
	return shared
}

func (r *Reboot) Name() string { return "reboot" }

func (r *Reboot) Entities() []esphome.Entity { return []esphome.Entity{r.button} }

func (r *Reboot) build() {
	r.button = &esphome.Button{
		Base: esphome.Base{
			ObjectID: "restart",
			Name:     "Restart",
			Icon:     "mdi:restart",
			Category: esphome.CategoryDiagnostic,
		},
		DeviceClass: "restart",

		OnPress: func() {
			safe.Go("reboot", func() {
				slog.Warn("rebooting, asked for in Home Assistant")
				if err := prop.Reboot(prop.Local); err != nil {
					slog.Error("rebooting failed", "err", err)
				}
			})
		},
	}
}
