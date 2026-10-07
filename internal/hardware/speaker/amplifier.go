package speaker

import "github.com/ygelfand/LANovo/internal/component"

func init() {
	component.Register(
		component.Hardware,
		func() Amplifier { return Amplifier{} },
		component.Order(56),
	)
}

type Amplifier struct{}

func (Amplifier) Name() string { return "amplifier" }

func (Amplifier) Startup() component.Progress {
	if p := Get().amp.Load(); p != nil {
		return *p
	}
	return component.Progress{Doing: "waiting for the speaker"}
}
