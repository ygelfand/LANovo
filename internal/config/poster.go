package config

import (
	"strings"

	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

type Poster = schema.Poster
type PosterEvery = schema.PosterEvery

const (
	PosterNever  = schema.PosterNever
	PosterWake   = schema.PosterWake
	PosterHourly = schema.PosterHourly
	PosterDaily  = schema.PosterDaily
)

var PosterEveries = schema.PosterEveries
var Names = schema.Names

func defaultPoster() Poster { return schema.DefaultPoster() }

type PosterWriter struct{ st *Store }

func (w PosterWriter) Enabled(v bool) error {
	return w.st.Update(func(c *Config) { c.Poster.Enabled = v })
}

func (w PosterWriter) Every(v PosterEvery) error {
	return w.st.Update(func(c *Config) { c.Poster.Every = v })
}

func (w PosterWriter) Server(v string) error {
	return w.st.Update(func(c *Config) { c.Poster.Server = strings.TrimSpace(v) })
}

func (w PosterWriter) Key(v string) error {
	return w.st.Update(func(c *Config) { c.Poster.Key = strings.TrimSpace(v) })
}

func (w PosterWriter) Albums(v string) error {
	return w.st.Update(func(c *Config) { c.Poster.Albums = v })
}

func (w PosterWriter) Tags(v string) error {
	return w.st.Update(func(c *Config) { c.Poster.Tags = v })
}

func (w PosterWriter) Last(v string) error {
	return w.st.Update(func(c *Config) { c.Poster.Last = v })
}
