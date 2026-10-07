package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type Visual = schema.Visual

var MaxFPSSteps = schema.MaxFPSSteps

const DefaultVisual = schema.DefaultVisual
const DefaultLabel = schema.DefaultLabel
const DefaultMaxFPS = schema.DefaultMaxFPS

var defaultVisual = schema.DefaultVisualSettings

type VisualWriter struct{ st *Store }

func (w VisualWriter) Kind(v string) error {
	return w.st.Update(func(c *Config) { c.Visual.Kind = v })
}

func (w VisualWriter) MaxFPS(v int) error {
	return w.st.Update(func(c *Config) { c.Visual.MaxFPS = v })
}

func (w VisualWriter) Seed(v int) error {
	return w.st.Update(func(c *Config) { c.Visual.Seed = v })
}

func (w VisualWriter) Label(v string) error {
	return w.st.Update(func(c *Config) { c.Visual.Label = v })
}
