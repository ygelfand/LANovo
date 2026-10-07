package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type Cast = schema.Cast
type YouTube = schema.YouTube
type Prime = schema.Prime

const LiveDelayLeast = schema.LiveDelayLeast
const LiveDelayMost = schema.LiveDelayMost

var defaultCast = schema.DefaultCast

// CastWriter changes it.
type CastWriter struct{ st *Store }

func (w CastWriter) Receiver(v bool) error {
	return w.st.Update(func(c *Config) { c.Cast.Receiver = v })
}

func (w CastWriter) Oracle(url string) error {
	return w.st.Update(func(c *Config) { c.Cast.Oracle = url })
}

func (w CastWriter) Screens(device, music, video string) error {
	return w.st.Update(func(c *Config) {
		c.Cast.YouTube.Device, c.Cast.YouTube.Music, c.Cast.YouTube.Video = device, music, video
	})
}

func (w CastWriter) PrimeSkipIntro(v bool) error {
	return w.st.Update(func(c *Config) { c.Cast.Prime.SkipIntro = v })
}

func (w CastWriter) PrimePersist(v bool) error {
	return w.st.Update(func(c *Config) { c.Cast.Prime.Persist = v })
}

func (w CastWriter) Skip(categories []string) error {
	if categories == nil {
		categories = []string{}
	}
	return w.st.Update(func(c *Config) { c.Cast.YouTube.Skip = categories })
}

func (w CastWriter) LiveDelay(seconds int) error {
	seconds = min(max(seconds, LiveDelayLeast), LiveDelayMost)
	return w.st.Update(func(c *Config) { c.Cast.YouTube.LiveDelay = seconds })
}

func (w CastWriter) LoungeOnDemand(v bool) error {
	return w.st.Update(func(c *Config) { c.Cast.YouTube.OnDemand = v })
}
