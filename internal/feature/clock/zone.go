package clock

import (
	"github.com/ygelfand/LANovo/internal/config"
	esphome "github.com/ygelfand/go-esphome-device"
	zonepolicy "github.com/ygelfand/libcountertop/pkg/settings/timezone"
)

const FollowHome = zonepolicy.FollowHome

func (c *Clock) Entities() []esphome.Entity { return []esphome.Entity{c.zone} }
func (c *Clock) Restore(config.Config)      { c.zonePolicy.Restore() }

var label = zonepolicy.Label

func (c *Clock) buildZone() {
	c.zonePolicy = zonepolicy.NewPolicy(zonepolicy.PolicyOptions{Read: func() config.Time { return config.Get().Time }, Home: func(s string) error { return config.Set().Time().Home(s) }, Chosen: func(s string) error { return config.Set().Time().Chosen(s) }})
	c.zone = c.zonePolicy.Select
}

var spec = zonepolicy.Spec

func (c *Clock) SetZone(s string)      { c.zonePolicy.SetZone(s) }
func (c *Clock) Zone() string          { return c.zonePolicy.Zone() }
func (c *Clock) zoneFromHome(s string) { c.zonePolicy.FromHome(s) }
