package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type Idle = schema.Idle
type IdleVisual = schema.IdleVisual
type Align = schema.Align
type Source = schema.Source

const (
	AlignLeft     = schema.AlignLeft
	AlignCenter   = schema.AlignCenter
	AlignRight    = schema.AlignRight
	SourceBoth    = schema.SourceBoth
	SourceMic     = schema.SourceMic
	SourceSpeaker = schema.SourceSpeaker
)

var IdleFaces = schema.IdleFaces
var Aligns = schema.Aligns
var Sources = schema.Sources
var defaultIdle = schema.DefaultIdle

type IdleWriter struct{ st *Store }

func (w IdleWriter) After(v Delay) error {
	return w.st.Update(func(c *Config) { c.Idle.After = v })
}

func (w IdleWriter) Media(v Delay) error {
	return w.st.Update(func(c *Config) { c.Idle.Media = v })
}

func (w IdleWriter) Face(v Face) error {
	return w.st.Update(func(c *Config) { c.Idle.Face = v })
}

func (w IdleWriter) Position(v Position) error {
	return w.st.Update(func(c *Config) { c.Idle.Position = v })
}

func (w IdleWriter) Align(v Align) error {
	return w.st.Update(func(c *Config) { c.Idle.Align = v })
}

func (w IdleWriter) Size(v Size) error {
	return w.st.Update(func(c *Config) { c.Idle.Size = v })
}

func (w IdleWriter) Visual(slot int, v IdleVisual) error {
	return w.st.Update(func(c *Config) {
		if slot == 0 {
			c.Idle.First = v
		} else {
			c.Idle.Second = v
		}
	})
}
