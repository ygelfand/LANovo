package config

import "github.com/ygelfand/LANovo/internal/lib/say"

type Call struct {
	Incoming   bool       `json:"incoming"`
	AutoAnswer bool       `json:"auto_answer"`
	AutoVideo  bool       `json:"auto_video"`
	PauseWake  bool       `json:"pause_wake"`
	Stream     CallStream `json:"stream"`
}

type CallStream string

const (
	CallMain CallStream = "main"
	CallSub  CallStream = "sub"
)

func CallStreams() []CallStream { return []CallStream{CallMain, CallSub} }

func (s CallStream) Label() string {
	if s == CallSub {
		return say.T("call.stream.sub")
	}
	return say.T("call.stream.main")
}

func defaultCall() Call { return Call{Incoming: true, PauseWake: true, Stream: CallMain} }

func (w CallWriter) Stream(v CallStream) error {
	return w.st.Update(func(c *Config) { c.Call.Stream = v })
}

func (w CallWriter) AutoVideo(v bool) error {
	return w.st.Update(func(c *Config) { c.Call.AutoVideo = v })
}

type CallWriter struct{ st *Store }

func (w CallWriter) Incoming(v bool) error {
	return w.st.Update(func(c *Config) { c.Call.Incoming = v })
}

func (w CallWriter) AutoAnswer(v bool) error {
	return w.st.Update(func(c *Config) { c.Call.AutoAnswer = v })
}

func (w CallWriter) PauseWake(v bool) error {
	return w.st.Update(func(c *Config) { c.Call.PauseWake = v })
}
