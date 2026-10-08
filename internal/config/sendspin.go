package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type Sendspin = schema.Sendspin

var defaultSendspin = schema.DefaultSendspin

type SendspinWriter struct{ st *Store }

func (w SendspinWriter) Enabled(v bool) error {
	return w.st.Update(func(c *Config) { c.Sendspin.Enabled = v })
}
