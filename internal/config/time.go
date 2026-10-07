package config

import (
	"github.com/ygelfand/libcountertop/pkg/settings/schema"
)

type Time = schema.Time

func defaultTime() Time { return Time{} }

// TimeWriter records how the device tells the time.
type TimeWriter struct{ st *Store }

// Home records what Home Assistant said, which is not necessarily what the device runs on.
func (w TimeWriter) Home(v string) error {
	return w.st.Update(func(c *Config) { c.Time.Home = v })
}

// Chosen sets the zone the device runs on whatever the server says. Empty gives it back.
func (w TimeWriter) Chosen(v string) error {
	return w.st.Update(func(c *Config) { c.Time.Chosen = v })
}
