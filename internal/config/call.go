package config

import "github.com/ygelfand/libcountertop/pkg/settings/schema"

type Call = schema.Call
type CallStream = schema.CallStream

const (
	CallMain = schema.CallMain
	CallSub  = schema.CallSub
)

var CallStreams = schema.CallStreams
var defaultCall = schema.DefaultCall

type CallWriter struct{ st *Store }

func (w CallWriter) Stream(v CallStream) error {
	return w.st.Update(func(c *Config) { c.Call.Stream = v })
}

func (w CallWriter) AutoVideo(v bool) error {
	return w.st.Update(func(c *Config) { c.Call.AutoVideo = v })
}

func (w CallWriter) Incoming(v bool) error {
	return w.st.Update(func(c *Config) { c.Call.Incoming = v })
}

func (w CallWriter) AutoAnswer(v bool) error {
	return w.st.Update(func(c *Config) { c.Call.AutoAnswer = v })
}

func (w CallWriter) PauseWake(v bool) error {
	return w.st.Update(func(c *Config) { c.Call.PauseWake = v })
}
