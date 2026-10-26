package diag

import (
	"sync"

	sharedcomponent "github.com/ygelfand/libcountertop/pkg/runtime/component"
	"github.com/ygelfand/libcountertop/pkg/system/diag"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/wakeword"
	"github.com/ygelfand/LANovo/internal/hardware/wifi"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/lib/wake"
)

func init() {
	component.Register(
		sharedcomponent.Network,
		func() componentDiag { return componentDiag{Get()} },
		sharedcomponent.Order(90),
	)
}

func zones() []diag.Zone {
	t := board.Current().Thermal
	var out []diag.Zone
	for _, z := range []diag.Zone{
		{ID: "cpu_temperature", Name: "CPU temperature", Zones: t.CPU},
		{ID: "gpu_temperature", Name: "GPU temperature", Zones: t.GPU},
	} {
		if len(z.Zones) > 0 {
			z.PerDegree = t.PerDegree
			out = append(out, z)
		}
	}
	return out
}

type componentDiag struct{ *diag.Diag }

func (c componentDiag) Restore(cfg config.Config) { c.Diag.Restore(cfg.Diag) }

var get = sync.OnceValue(func() *diag.Diag {
	d := diag.New(diag.Options{
		Settings: config.DiagSection,
		StateDir: layout.StateDir,
		Version:  layout.VersionString(),
		Zones:    zones(),
		Cache:    wake.Lib().CacheEntities(inUse),
		Network:  wifi.Get().Network,
		Signal:   wifi.Get().Signal,
		LogTag:   layout.LogTag,
	})
	component.Settings.Add(d.Controls())
	component.Subscribed.Listen(func(struct{}) { d.Soon() })
	return d
})

func Get() *diag.Diag { return get() }

func inUse() []string { return config.Get().Wake.IDs(wakeword.Slots) }
