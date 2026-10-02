package gui

import (
	"math"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"
)

const (
	tapDelay   = 100 * time.Millisecond
	pressFlash = 64 * time.Millisecond
	rippleGrow = 300 * time.Millisecond
	rippleFade = 250 * time.Millisecond
	rippleTick = 16 * time.Millisecond
)

type ripple struct {
	id         string
	x, y, w, h float32
	lit, up    time.Time
	gen        int
}

var (
	armed   string
	pending func(gogui.EventCtx)
	wave    ripple
	waveGen int
	moving  bool
)

func (r ripple) done(now time.Time) bool {
	return r.id == "" || r.lit.IsZero() || !r.up.IsZero() && now.Sub(r.up) >= rippleFade
}

func (r ripple) still(now time.Time) bool {
	return r.done(now) || r.up.IsZero() && now.Sub(r.lit) >= rippleGrow
}

func animate(w *gogui.Window) {
	if moving {
		return
	}
	moving = true
	var step func()
	step = func() {
		w.QueueCommand(func(w *gogui.Window) {
			w.InvalidateLayout()
			if wave.still(time.Now()) {
				moving = false
				return
			}
			time.AfterFunc(rippleTick, step)
		})
	}
	step()
}

func light(w *gogui.Window, gen int) {
	if wave.gen != gen || !wave.lit.IsZero() {
		return
	}
	wave.lit = time.Now()
	animate(w)
}

func release(e gogui.EventCtx) {
	if armed == "" {
		return
	}
	armed, pending = "", nil
	if !wave.lit.IsZero() && wave.up.IsZero() {
		wave.up = time.Now()
		animate(e.Window)
	} else {
		wave = ripple{}
	}
	e.Window.InvalidateLayout()
}

func overlay(r ripple, now time.Time) gogui.View {
	t := gogui.CurrentTheme().Cfg
	reach := float32(math.Hypot(float64(max(r.x, r.w-r.x)), float64(max(r.y, r.h-r.y))))
	grown := min(float32(now.Sub(r.lit))/float32(rippleGrow), 1)
	grown = 1 - (1-grown)*(1-grown)
	alpha := float32(1)
	if !r.up.IsZero() {
		alpha = max(1-float32(now.Sub(r.up))/float32(rippleFade), 0)
	}
	ink := t.TextStyleDef.Color
	ink.A = uint8(48 * min(alpha, 1))
	return gogui.DrawCanvas(gogui.DrawCanvasCfg{
		Width: r.w, Height: r.h, Clip: true, Radius: t.RadiusMedium,
		Version: uint64(now.UnixNano()),
		OnDraw: func(dc *gogui.DrawContext) {
			if grown >= 1 {
				dc.FilledRoundedRect(0, 0, r.w, r.h, t.RadiusMedium, ink)
				return
			}
			dc.FilledPolygon(disc(r.x, r.y, reach*grown), ink)
		},
	})
}

func disc(cx, cy, radius float32) []float32 {
	const sides = 96
	points := make([]float32, 0, sides*2)
	for i := range sides {
		a := 2 * math.Pi * float64(i) / sides
		points = append(points, cx+radius*float32(math.Cos(a)), cy+radius*float32(math.Sin(a)))
	}
	return points
}

func pressable(layout func(gogui.ContainerCfg) gogui.View, cfg gogui.ContainerCfg, do func(gogui.EventCtx)) gogui.View {
	id := cfg.ID
	if do == nil || cfg.Disabled {
		return layout(cfg)
	}
	if now := time.Now(); wave.id == id && !wave.done(now) {
		cfg.Content = append(cfg.Content, gogui.Column(gogui.ContainerCfg{
			Float: true, Padding: gogui.NoPadding,
			Content: []gogui.View{overlay(wave, now)},
		}))
	}
	cfg.OnClick = func(e gogui.EventCtx) {
		armed = id
		waveGen++
		gen := waveGen
		s := e.Layout.Shape
		wave = ripple{id: id, x: e.Event.MouseX - s.X, y: e.Event.MouseY - s.Y, w: s.Width, h: s.Height, gen: gen}
		pending = do
		w := e.Window
		time.AfterFunc(tapDelay, func() { w.QueueCommand(func(w *gogui.Window) { light(w, gen) }) })
		e.Consume()
	}
	fire := func(e gogui.EventCtx) {
		if armed == id {
			lift(e.Window)
			e.Consume()
		}
	}
	cfg.OnMouseUp = fire
	cfg.OnGesture = func(e gogui.EventCtx) {
		switch {
		case e.Event == nil:
		case e.Event.GestureType == gogui.GesturePan:
			if armed == id {
				release(e)
			}
		case e.Event.GestureType == gogui.GestureTap,
			e.Event.GestureType == gogui.GestureLongPress && e.Event.GesturePhase == gogui.GesturePhaseEnded:
			fire(e)
		}
	}
	return layout(cfg)
}

func lift(w *gogui.Window) {
	if armed == "" {
		return
	}
	armed = ""
	do := pending
	pending = nil
	if !wave.lit.IsZero() {
		wave.up = time.Now()
		animate(w)
		do(gogui.EventCtx{Window: w})
		w.InvalidateLayout()
		return
	}
	wave.lit = time.Now()
	wave.up = wave.lit.Add(pressFlash)
	animate(w)
	gen := wave.gen
	time.AfterFunc(pressFlash, func() {
		w.QueueCommand(func(w *gogui.Window) {
			if wave.gen == gen {
				do(gogui.EventCtx{Window: w})
				w.InvalidateLayout()
			}
		})
	})
}

func tapped(tap func(int), level int) func(gogui.EventCtx) {
	if tap == nil {
		return nil
	}
	return func(gogui.EventCtx) { tap(level) }
}

func button(id string, glyph string, st gogui.TextStyle, fill gogui.Color, do func(gogui.EventCtx)) gogui.View {
	side := st.Size * 2
	return pressable(gogui.Row, gogui.ContainerCfg{
		ID:      id,
		Width:   side,
		Height:  side,
		Sizing:  gogui.FixedFixed,
		Color:   fill,
		Radius:  gogui.RadiusLarge,
		Padding: gogui.NoPadding,
		HAlign:  gogui.HAlignCenter,
		VAlign:  gogui.VAlignMiddle,
		Content: []gogui.View{gogui.Label(glyph, st)},
	}, do)
}
