package idle

import (
	"errors"
	"log/slog"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/gpu"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

const shadeEvery = time.Second / 60

type shading struct {
	layer  *gpu.Layer
	vis    visual.Visual
	source config.Source
	area   ui.Rect
	rot    display.Orientation
	fresh  bool
	last   time.Time
}

func (v *View) shaded(n int, vis visual.Visual, area ui.Rect, source config.Source) {
	v.gmu.Lock()
	defer v.gmu.Unlock()
	if v.broken[n] == vis {
		return
	}
	g := v.gl[n]
	if g != nil && (g.vis != vis || g.area.W != area.W || g.area.H != area.H || !g.layer.Alive()) {
		v.unshade(n)
		g = nil
	}
	if g == nil {
		l, err := gpu.Open(area.W, area.H)
		if errors.Is(err, gpu.ErrNoHelper) {
			return
		}
		if err != nil {
			slog.Error("idle: opening the GPU layer", "err", err)
			v.broken[n] = vis
			return
		}
		g = &shading{layer: l, vis: vis, fresh: true}
		v.gl[n] = g
	}
	g.source = source
	if rot := display.Get().Orientation(); g.area != area || g.rot != rot {
		if err := g.layer.Place(area); err != nil {
			if g.layer.Alive() {
				slog.Error("idle: placing the GPU layer", "err", err)
				v.broken[n] = vis
			}
			v.unshade(n)
			return
		}
		g.area, g.rot = area, rot
	}
}

func (v *View) unshade(n int) {
	if g := v.gl[n]; g != nil {
		g.layer.Close()
		v.gl[n] = nil
	}
}

func (v *View) unshadeAll() {
	v.gmu.Lock()
	defer v.gmu.Unlock()
	for n := range v.gl {
		v.unshade(n)
	}
}

func (v *View) shade() {
	v.gmu.Lock()
	defer v.gmu.Unlock()
	var x visual.Input
	for n, g := range v.gl {
		if g == nil {
			continue
		}
		if x.Now == 0 {
			x = visuals.Get().Input()
		}
		now := time.Now()
		in := visuals.From(x, g.source)
		if !g.last.IsZero() {
			in.Dt = now.Sub(g.last)
		}
		g.last = now
		if err := g.vis.Shade(g.layer, g.fresh, g.area, in); err != nil {
			if g.layer.Alive() {
				slog.Error("idle: GPU visual failed", "err", err)
				v.broken[n] = g.vis
			}
			v.unshade(n)
			continue
		}
		if g.fresh {
			slog.Info("idle: visual ready", "slot", n, "took", time.Since(now).Round(time.Millisecond))
		}
		g.fresh = false
	}
}

func (v *View) painting(slots int) bool {
	v.gmu.Lock()
	defer v.gmu.Unlock()
	for n := range slots {
		if v.gl[n] == nil && v.broken[n] == nil {
			return true
		}
	}
	return false
}
