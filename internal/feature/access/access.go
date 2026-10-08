package access

import (
	"log/slog"
	"strconv"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
)

func init() {
	component.Register(sharedcomponent.Device, Get, sharedcomponent.Order(60))
}

const (
	port = "service.adb.tcp.port"
	off  = "-1"

	service = "adbd"
)

type Access struct{ adb *esphome.Switch }

var (
	once   sync.Once
	shared *Access
)

func Get() *Access {
	once.Do(func() {
		shared = &Access{}
		shared.build()
	})
	return shared
}

func (a *Access) Name() string { return "access" }

func (a *Access) Entities() []esphome.Entity { return []esphome.Entity{a.adb} }

// The adb TCP port property is volatile; a reboot clears it.
func (a *Access) Restore(cfg config.Config) {
	if !cfg.Access.ADB {
		return
	}

	if at, err := prop.Local.Getprop(port); err == nil && at == strconv.Itoa(config.ADBPort) {
		a.adb.Set(true)
		slog.Info("adb is already on the network", "port", config.ADBPort)
		return
	}
	a.apply(true)
}

func (a *Access) ADB() bool { return a.adb.Get() }

func (a *Access) SetADB(on bool) {
	a.apply(on)

	if err := config.Set().Access().ADB(on); err != nil {
		slog.Error("saving a setting failed", "setting", a.adb.ObjectID, "err", err)
	}
}

// adbd reads the property only at start.
func (a *Access) apply(on bool) {
	value := off
	if on {
		value = strconv.Itoa(config.ADBPort)
	}

	if err := prop.Local.Setprop(port, value); err != nil {
		slog.Error("adb over the network", "setting", port, "err", err)
		return
	}
	if err := prop.Restart(prop.Local, service); err != nil {
		slog.Error("adb over the network", "restarting", service, "err", err)
		return
	}

	a.adb.Set(on)

	if on {
		slog.Warn(
			"adb is listening on the network: anything that can reach this device has a root shell",
			"port",
			config.ADBPort,
		)
	} else {
		slog.Info("adb is back to the cable only")
	}
}

func (a *Access) build() {
	a.adb = &esphome.Switch{
		Base: esphome.Base{
			ObjectID: "adb_over_network",
			Name:     "ADB over network",
			Icon:     "mdi:console-network",
			Category: esphome.CategoryConfig,
		},
	}

	a.adb.OnCommand = a.SetADB
}
