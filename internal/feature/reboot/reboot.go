// Package reboot is the one thing Home Assistant can ask for that ends this process for good: a
// reboot of the device.
package reboot

import (
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"
)

func init() {
	component.Register(component.Device, Get, component.Order(90))
}

// Reboot asks init to bring the whole device back.
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

		// On its own goroutine: the press is answered before the device goes away, so Home
		// Assistant sees the button worked rather than the connection dropping.
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
