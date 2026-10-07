package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"
import "maps"

type Home = schema.Home
type HomePick = schema.HomePick

const (
	HomeGroupDefault = schema.HomeGroupDefault
	HomeGroupMost    = schema.HomeGroupMost
)

var defaultHome = schema.DefaultHome

type HomeWriter struct{ st *Store }

func (w HomeWriter) Control(key string, on bool) error {
	return w.st.Update(func(c *Config) {
		control := maps.Clone(c.Home.Control)
		if control == nil {
			control = map[string]bool{}
		}
		if on {
			control[key] = true
		} else {
			delete(control, key)
		}
		c.Home.Control = control
	})
}

func (w HomeWriter) Group(key string, n int) error {
	return w.st.Update(func(c *Config) {
		group := maps.Clone(c.Home.Group)
		if group == nil {
			group = map[string]int{}
		}
		group[key] = min(max(n, 0), HomeGroupMost)
		c.Home.Group = group
	})
}

func (w HomeWriter) Enabled(v bool) error {
	return w.st.Update(func(c *Config) { c.Home.Enabled = v })
}

func (w HomeWriter) Combine(v bool) error {
	return w.st.Update(func(c *Config) { c.Home.Combine = v })
}

func (w HomeWriter) Pick(key string, p HomePick) error {
	return w.st.Update(func(c *Config) {
		picks := maps.Clone(c.Home.Picks)
		if picks == nil {
			picks = map[string]HomePick{}
		}
		if p.Empty() {
			delete(picks, key)
		} else {
			picks[key] = p.Clone()
		}
		c.Home.Picks = picks
	})
}
