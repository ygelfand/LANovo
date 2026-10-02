package clock

import (
	"log/slog"
	"time"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/lib/tz"
)

// Where the device is. Home Assistant sends it with the time and is right almost always, but a
// device does not have to be in the same place as its server — and ESPHome has only a build-time
// timezone key, so there is nowhere to say otherwise. A zone chosen here outranks the server's.

// FollowHome is what the zone select offers for giving the choice back to the server. Not a zone
// name, so it cannot collide with one.
const FollowHome = "Home Assistant"

// Entities is the one setting this feature has.
func (c *Clock) Entities() []esphome.Entity { return []esphome.Entity{c.zone} }

// Restore puts the device back in its zone before anything shows a time.
func (c *Clock) Restore(cfg config.Config) {
	if err := tz.Use(spec(cfg.Time)); err != nil {
		slog.Warn("could not use the saved time zone", "zone", spec(cfg.Time), "err", err)
	}
	c.zone.Set(label(cfg.Time.Chosen))
}

// label is how a stored choice reads in the select, where empty means the server decides.
func label(chosen string) string {
	if chosen == "" {
		return FollowHome
	}
	return chosen
}

// buildZone is the select of everywhere the device can be told it is.
func (c *Clock) buildZone() {
	c.zone = &esphome.Select{
		Base: esphome.Base{
			ObjectID: "time_zone",
			Name:     "Time zone",
			Icon:     "mdi:map-clock",
			Category: esphome.CategoryConfig,
		},
		Options: append([]string{FollowHome}, tz.Names()...),
	}

	c.zone.OnCommand = func(name string) {
		if name == FollowHome {
			name = ""
		}
		c.SetZone(name)
	}
}

// spec is the POSIX string to keep time by: the rules for whatever was chosen here, or failing
// that whatever Home Assistant last sent.
//
// A chosen name that is no longer offered falls back rather than leaving the device on UTC, which
// is what dropping a zone from the list would otherwise do to anyone who had picked it.
func spec(t config.Time) string {
	if t.Chosen != "" {
		if z, ok := tz.Named(t.Chosen); ok {
			return z.Spec
		}
		slog.Warn("the chosen time zone is not one we know", "zone", t.Chosen)
	}
	return t.Home
}

// SetZone takes a zone by IANA name and keeps the device on it whatever Home Assistant says. An
// empty name gives the choice back to the server.
func (c *Clock) SetZone(name string) {
	if name != "" {
		if _, ok := tz.Named(name); !ok {
			slog.Warn("no such time zone", "zone", name)
			return
		}
	}

	if err := config.Set().Time().Chosen(name); err != nil {
		slog.Error("saving the time zone failed", "err", err)
		return
	}

	cfg := config.Get()
	if err := tz.Use(spec(cfg.Time)); err != nil {
		slog.Warn("could not use the chosen time zone", "zone", name, "err", err)
		return
	}
	slog.Info("time zone", "chosen", name, "now", time.Now().Format(time.RFC1123))
}

// Zone is the IANA name the device is set to, empty when it is following Home Assistant.
func (c *Clock) Zone() string { return config.Get().Time.Chosen }

// zoneFromHome remembers what Home Assistant sent, and applies it unless something here outranks
// it. Remembering either way, so giving the choice back does not need a reconnection to take
// effect.
//
// It arrives as a POSIX TZ string, which every version of Home Assistant sends. The ParsedTimezone
// beside it has only been there since February 2026, and carries the same rules with the offsets
// signed the other way round.
func (c *Clock) zoneFromHome(said string) {
	if said == "" {
		return
	}

	if said != config.Get().Time.Home {
		if err := config.Set().Time().Home(said); err != nil {
			slog.Error("saving the time zone failed", "err", err)
		}
	}

	want := spec(config.Get().Time)
	if want == "" || want == tz.Current() {
		return
	}

	if err := tz.Use(want); err != nil {
		slog.Warn("could not take the time zone from Home Assistant", "zone", want, "err", err)
		return
	}
	slog.Info("time zone", "zone", want, "now", time.Now().Format(time.RFC1123))
}
