package access

import (
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/say"
	setting "github.com/ygelfand/libcountertop/pkg/settings"

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

const Group setting.Group = "Access"

type Access struct {
	knobs *setting.Controls[config.Access]
}

var (
	once   sync.Once
	shared *Access
)

func Get() *Access {
	once.Do(func() {
		shared = &Access{}
		shared.knobs = &setting.Controls[config.Access]{
			Table: Table(),
			Read:  func() config.Access { return config.Get().Access },
			Save:  shared.store,
		}
		component.Settings.Add(shared.knobs)
	})
	return shared
}

func Table() *setting.Table[config.Access] {
	return setting.NewTable("access", []setting.Group{Group}, []setting.Setting[config.Access]{
		setting.Flag(
			setting.Setting[config.Access]{
				Name:  "adb",
				Group: Group,
				ID:    "adb_over_network",
				Icon:  "mdi:console-network",
			},
			func(c *config.Access) *bool { return &c.ADB },
		),
	}, setting.Messages{Text: say.T, Missing: say.Missing})
}

func (a *Access) Name() string { return "access" }

func (a *Access) Entities() []esphome.Entity { return a.knobs.Entities() }

// The adb TCP port property is volatile; a reboot clears it.
func (a *Access) Restore(cfg config.Config) {
	a.knobs.PublishFrom(cfg.Access)
	if !cfg.Access.ADB {
		return
	}

	if at, err := prop.Local.Getprop(port); err == nil && at == strconv.Itoa(config.ADBPort) {
		slog.Info("adb is already on the network", "port", config.ADBPort)
		return
	}
	if err := apply(true); err != nil {
		slog.Error("adb over the network", "err", err)
	}
}

func (a *Access) ADB() bool { return config.Get().Access.ADB }

func (a *Access) SetADB(on bool) {
	if err := a.knobs.Change("adb", setting.OnOff(on)); err != nil {
		slog.Error("saving a setting failed", "setting", "access.adb", "err", err)
	}
}

func (a *Access) store(s setting.Setting[config.Access], v string) error {
	at := config.Get().Access
	if err := s.Write(&at, v); err != nil {
		return err
	}
	if err := apply(at.ADB); err != nil {
		return err
	}
	return setting.Store(config.AccessSection, s, v)
}

// adbd reads the property only at start.
func apply(on bool) error {
	value := off
	if on {
		value = strconv.Itoa(config.ADBPort)
	}

	if err := prop.Local.Setprop(port, value); err != nil {
		return fmt.Errorf("setting %s: %w", port, err)
	}
	if err := prop.Restart(prop.Local, service); err != nil {
		return fmt.Errorf("restarting %s: %w", service, err)
	}

	if on {
		slog.Warn(
			"adb is listening on the network: anything that can reach this device has a root shell",
			"port",
			config.ADBPort,
		)
	} else {
		slog.Info("adb is back to the cable only")
	}
	return nil
}
