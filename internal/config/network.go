package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type Network = schema.Network

const DefaultVerify = true

var defaultNetwork = schema.DefaultNetwork

type NetworkWriter struct{ st *Store }

func (w NetworkWriter) Verify(v bool) error {
	return w.st.Update(func(c *Config) { c.Network.Verify = v })
}
func (w NetworkWriter) Address(v string) error {
	return w.st.Update(func(c *Config) { c.Network.Address = v })
}
