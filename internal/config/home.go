package config

import (
	"maps"
	"slices"
)

type Home struct {
	Picks   map[string]HomePick `json:"picks"`
	Control map[string]bool     `json:"control"`
	Combine bool                `json:"combine"`
}

type HomePick struct {
	All      bool     `json:"all"`
	Labels   []string `json:"labels"`
	Areas    []string `json:"areas"`
	Entities []string `json:"entities"`
}

func (p HomePick) Empty() bool {
	return !p.All && len(p.Labels) == 0 && len(p.Areas) == 0 && len(p.Entities) == 0
}

func (p HomePick) clone() HomePick {
	p.Labels = slices.Clone(p.Labels)
	p.Areas = slices.Clone(p.Areas)
	p.Entities = slices.Clone(p.Entities)
	return p
}

func defaultHome() Home { return Home{} }

func (h Home) Picked(key string) HomePick { return h.Picks[key] }

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
			picks[key] = p.clone()
		}
		c.Home.Picks = picks
	})
}
