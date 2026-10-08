package config

import (
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

type Time = schema.Time

func defaultTime() Time { return Time{} }

type TimeWriter struct{ st *Store }

func (w TimeWriter) Home(v string) error {
	return w.st.Update(func(c *Config) { c.Time.Home = v })
}

func (w TimeWriter) Chosen(v string) error {
	return w.st.Update(func(c *Config) { c.Time.Chosen = v })
}
