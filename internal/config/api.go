package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type API = schema.API

var defaultAPI = schema.DefaultAPI

type APIWriter struct{ st *Store }

func (w APIWriter) Adopted(v bool) error {
	return w.st.Update(func(c *Config) { c.API.Adopted = v })
}
