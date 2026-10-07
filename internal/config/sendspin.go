package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

// Sendspin is the room's part in whole-house audio.
type Sendspin = schema.Sendspin

var defaultSendspin = schema.DefaultSendspin

// SendspinWriter changes the room's part in whole-house audio.
type SendspinWriter struct{ st *Store }

func (w SendspinWriter) Enabled(v bool) error {
	return w.st.Update(func(c *Config) { c.Sendspin.Enabled = v })
}
