package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type RTSP = schema.RTSP

var defaultRTSP = schema.DefaultRTSP

type RTSPWriter struct{ st *Store }

func (w RTSPWriter) Enabled(v bool) error {
	return w.st.Update(func(c *Config) { c.RTSP.Enabled = v })
}
