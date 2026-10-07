// Package access is what the device leaves open to the network, which today is one question:
// whether adbd listens on TCP.
//
// adb over the network is an unauthenticated root shell — ro.secure is 0 on this device, so there
// is no key prompt behind it.
package access

import (
	"log/slog"
	"strconv"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/android/prop"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
)

func init() {
	component.Register(component.Device, Get, component.Order(60))
}

// port is the property adbd reads at start-up, and off is the value that means USB only.
const (
	port = "service.adb.tcp.port"
	off  = "-1"

	// service is what init calls adbd, for the restart that makes it read the property again.
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

// Restore turns adb back on if that is how the device was left. The property is volatile, so a
// reboot has already turned it off and this is the only thing that brings it back.
//
// Nothing is turned off here. Off is the state a boot arrives in, and a restart of lanovod that
// bounced adbd would cut the session somebody is most likely using to watch it start.
func (a *Access) Restore(cfg config.Config) {
	if !cfg.Access.ADB {
		return
	}

	// The property still holding the port means adbd has already read it, so this is a restart of
	// lanovod rather than a boot. Restarting adbd here cuts the session somebody is most likely
	// using to watch this start — which is what it did, on every restart, until this was here.
	if at, err := prop.Local.Getprop(port); err == nil && at == strconv.Itoa(config.ADBPort) {
		a.adb.Set(true)
		slog.Info("adb is already on the network", "port", config.ADBPort)
		return
	}
	a.apply(true)
}

// ADB reports whether adb over the network is on.
func (a *Access) ADB() bool { return a.adb.Get() }

// SetADB opens or closes adb over the network and remembers which. Everything that changes it comes
// through here.
func (a *Access) SetADB(on bool) {
	a.apply(on)

	if err := config.Set().Access().ADB(on); err != nil {
		slog.Error("saving a setting failed", "setting", a.adb.ObjectID, "err", err)
	}
}

// apply sets the property and restarts adbd, which is the only time it reads it.
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
