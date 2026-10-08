package firmware

import (
	"context"
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/hook"
	"github.com/ygelfand/libcountertop/pkg/runtime/safe"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/feedback"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/update"
)

func init() {
	component.Register(component.Network, Get, component.Order(10))
}

const (
	EventInstalled = "installed"
	EventFailed    = "failed"
)

type Upgrade struct {
	Version string
	At      float32
}

func (up Upgrade) Active() bool { return up.Version != "" }

type Firmware struct {
	Upgrading hook.Hook[Upgrade]

	entity  *esphome.Update
	channel *esphome.Select
	look    *esphome.Button
	status  *esphome.TextSensor
	events  *esphome.Event

	mu      sync.Mutex
	found   update.Manifest
	upgrade Upgrade

	announced     sync.Once
	rebootPending bool
}

var (
	once   sync.Once
	shared *Firmware
)

func Get() *Firmware {
	once.Do(func() { shared = build() })
	return shared
}

func build() *Firmware {
	u := &Firmware{}

	u.entity = &esphome.Update{
		Base: esphome.Base{
			ObjectID: "firmware",
			Name:     "Firmware",
			Icon:     "mdi:package-up",
		},
		DeviceClass: "firmware",
		OnCommand:   u.command,
	}
	u.publish(update.Manifest{Version: layout.Version})

	u.channel = &esphome.Select{
		Base: esphome.Base{
			ObjectID: "update_channel",
			Name:     "Update channel",
			Icon:     "mdi:source-branch",
			Category: esphome.CategoryDiagnostic,
		},
		Options: config.Labels(update.Channels()),
	}
	u.channel.OnCommand = func(label string) {
		c, ok := config.ByLabel(update.Channels(), label)
		if !ok {
			return
		}
		u.channel.Set(c.Label())
		if err := config.Set().Update().Channel(c.Label()); err != nil {
			slog.Error("saving a setting failed", "setting", u.channel.ObjectID, "err", err)
		}
		safe.Go("update check", func() { u.Check(context.Background()) })
	}

	u.look = &esphome.Button{
		Base: esphome.Base{
			ObjectID: "check_for_updates",
			Name:     "Check for updates",
			Icon:     "mdi:cloud-search",
			Category: esphome.CategoryDiagnostic,
		},
		OnPress: func() { safe.Go("update check", func() { u.Check(context.Background()) }) },
	}

	u.status = &esphome.TextSensor{
		Base: esphome.Base{
			ObjectID: "update_status",
			Name:     "Update status",
			Icon:     "mdi:package-variant",
			Category: esphome.CategoryDiagnostic,
		},
	}

	u.events = &esphome.Event{
		Base: esphome.Base{
			ObjectID: "update_outcome",
			Name:     "Update outcome",
			Icon:     "mdi:package-up",
		},
		Types: []string{EventInstalled, EventFailed},
	}

	component.Subscribed.Listen(func(struct{}) { safe.Go("update announce", u.announce) })
	return u
}

func (u *Firmware) Restore(c config.Config) { u.channel.Set(u.Channel().Label()) }

func (u *Firmware) RebootPending(pending bool) {
	u.mu.Lock()
	u.rebootPending = pending
	u.mu.Unlock()
}

func (u *Firmware) announce() {
	u.announced.Do(func() {
		u.mu.Lock()
		pending := u.rebootPending
		u.mu.Unlock()

		if pending {
			u.status.Set(component.Fit("installed " + layout.Version + ", reboot to finish"))
		}
		if config.Get().Update.LastVersion == layout.Version {
			return
		}
		u.Settled(EventInstalled, "installed "+layout.Version)
		if pending {
			u.status.Set(component.Fit("installed " + layout.Version + ", reboot to finish"))
		}
		if err := config.Set().Update().LastVersion(layout.Version); err != nil {
			slog.Error("saving the reported version failed", "err", err)
		}
	})
}

func (u *Firmware) Settled(event, status string) {
	u.status.Set(component.Fit(status))
	u.events.Trigger(event)
}

func (u *Firmware) Channel() update.Channel {
	if c, ok := config.ByLabel(update.Channels(), config.Get().Update.Channel); ok {
		return c
	}
	return update.Stable
}

func (u *Firmware) Check(ctx context.Context) {
	channel := u.Channel()

	found, err := update.Fetch(ctx, channel)
	if err != nil {
		slog.Error("checking for an update failed", "channel", channel.Label(), "err", err)
		return
	}

	u.mu.Lock()
	u.found = found
	u.mu.Unlock()

	slog.Info(
		"update check",
		"channel",
		channel.Label(),
		"running",
		layout.Version,
		"offered",
		found.Version,
	)
	u.publish(found)
}

func (u *Firmware) command(cmd esphome.UpdateCommand) {
	switch cmd {
	case esphome.UpdateCheck:
		safe.Go("update check", func() { u.Check(context.Background()) })
	case esphome.UpdateInstall:
		safe.Go("update install", func() { u.Install(context.Background()) })
	}
}

func (u *Firmware) Install(ctx context.Context) {
	found, err := update.Fetch(ctx, u.Channel())
	u.mu.Lock()
	if err != nil {
		slog.Warn("re-reading the channel failed, using the last check", "err", err)
		found = u.found
	} else {
		u.found = found
	}
	u.mu.Unlock()

	if found.Version == "" || found.Version == layout.Version {
		slog.Warn("an install was asked for with nothing to install", "running", layout.Version)
		return
	}

	u.progress(found, 0)
	err = update.Install(ctx, found, func(at float32) { u.progress(found, at) })
	if err != nil {
		slog.Error("installing an update failed", "version", found.Version, "err", err)
		u.upgrading(Upgrade{})
		u.publish(found)
		u.Settled(EventFailed, "installing "+found.Version+" failed: "+err.Error())
		feedback.Failure()
		return
	}
	update.Restart("update to " + found.Version)
}

func (u *Firmware) progress(found update.Manifest, at float32) {
	state := u.state(found)
	state.InProgress, state.Progress = true, at*100
	u.entity.Set(state)
	u.upgrading(Upgrade{Version: found.Version, At: at})
}

func (u *Firmware) upgrading(up Upgrade) {
	u.mu.Lock()
	u.upgrade = up
	u.mu.Unlock()
	u.Upgrading.Emit(up)
}

func (u *Firmware) Upgrade() Upgrade {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.upgrade
}

func (u *Firmware) publish(found update.Manifest) { u.entity.Set(u.state(found)) }

func (u *Firmware) state(found update.Manifest) esphome.UpdateState {
	return esphome.UpdateState{
		CurrentVersion: layout.Version,
		LatestVersion:  found.Version,
		Title:          "LANovo (" + u.Channel().Label() + " channel)",
		ReleaseSummary: found.Notes,
		ReleaseURL:     found.ReleaseURL,
	}
}

func (u *Firmware) Name() string { return "firmware" }

func (u *Firmware) Entities() []esphome.Entity {
	return []esphome.Entity{u.entity, u.channel, u.look, u.status, u.events}
}
