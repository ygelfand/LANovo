package speaker

import (
	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"

	"github.com/ygelfand/LANovo/internal/component"
)

func init() {
	component.Register(
		sharedcomponent.Hardware,
		func() Amplifier { return Amplifier{} },
		sharedcomponent.Order(56),
	)
}

type Amplifier struct{}

func (Amplifier) Name() string { return "amplifier" }

func (Amplifier) Startup() sharedcomponent.Progress {
	if p := Get().amp.Load(); p != nil {
		return *p
	}
	return sharedcomponent.Progress{Doing: "waiting for the speaker"}
}
