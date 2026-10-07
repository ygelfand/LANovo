package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type API = schema.API

var defaultAPI = schema.DefaultAPI

// APIWriter records what the device knows about Home Assistant.
type APIWriter struct{ st *Store }

func (w APIWriter) Adopted(v bool) error {
	return w.st.Update(func(c *Config) { c.API.Adopted = v })
}
