package idle

import (
	"fmt"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/dashboard/face"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/visuals"
	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

const (
	frameEvery = time.Second / 20
	forever    = 100 * 365 * 24 * time.Hour
)

type View struct {
	mu      sync.Mutex
	vis     [2]visual.Visual
	kinds   [2]string
	running bool
	shown   string

	gmu    sync.Mutex
	gl     [2]*shading
	broken [2]visual.Visual
}

func newView() *View { return &View{} }

func (v *View) Covers() bool           { return true }
func (v *View) Asleep() bool           { return true }
func (v *View) Timeout() time.Duration { return forever }
func (v *View) Keep(w, h int) []config.IdleVisual {
	cfg := config.Get()
	slots := chosen(cfg.Idle)

	v.mu.Lock()
	if !v.running {
		v.running = true
		go v.run()
	}
	var vis []visual.Visual
	for n, slot := range slots {
		if v.vis[n] == nil || v.kinds[n] != slot.Kind {
			v.vis[n], v.kinds[n] = visual.New(visual.Kind(slot.Kind)), slot.Kind
		}
		vis = append(vis, v.vis[n])
	}
	v.shown = key(cfg, time.Now())
	v.mu.Unlock()

	for n, area := range Areas(w, h, len(slots)) {
		v.shaded(n, vis[n], area, slots[n].Source)
	}
	return slots
}

func chosen(c config.Idle) []config.IdleVisual {
	var out []config.IdleVisual
	for _, v := range []config.IdleVisual{c.First, c.Second} {
		if v.On() {
			out = append(out, v)
		}
	}
	return out
}

func Areas(w, h, n int) []ui.Rect {
	switch {
	case n <= 0:
		return nil
	case n == 1:
		return []ui.Rect{{W: w, H: h}}
	case w > h:
		return []ui.Rect{{W: w / 2, H: h}, {X: w / 2, W: w - w/2, H: h}}
	}
	return []ui.Rect{{W: w, H: h / 2}, {Y: h / 2, W: w, H: h - h/2}}
}

func Reading(cfg config.Config, at time.Time) face.Reading {
	r := face.Read(at, cfg.Screen.Hours == config.TwentyFourHour)
	if !cfg.Clock.Date {
		r = r.Undated()
	}
	return r
}

func key(cfg config.Config, at time.Time) string {
	r := Reading(cfg, at)
	k := fmt.Sprintf("%v %+v ink=%s", r, cfg.Idle, cfg.Clock.Ink)
	if face.Ticks(cfg.Idle.Face) {
		k = fmt.Sprintf("%s second=%d", k, r.Second)
	}
	return k
}

type Piece struct {
	Clip    ui.Rect
	Palette theme.Theme
}

func Pieces(palette theme.Theme, tones []visual.Traits, areas []ui.Rect, box ui.Rect) []Piece {
	var out []Piece
	for n, area := range areas {
		if n >= len(tones) {
			break
		}
		if clip := intersect(area, box); clip.W > 0 {
			out = append(out, Piece{Clip: clip, Palette: tones[n].Under(palette)})
		}
	}
	if len(out) == 0 {
		return []Piece{{Palette: palette}}
	}
	if len(out) == 2 && out[0].Palette == out[1].Palette {
		return []Piece{{Palette: out[0].Palette}}
	}
	return out
}

func intersect(a, b ui.Rect) ui.Rect {
	x0, y0 := max(a.X, b.X), max(a.Y, b.Y)
	x1, y1 := min(a.X+a.W, b.X+b.W), min(a.Y+a.H, b.Y+b.H)
	if x1 <= x0 || y1 <= y0 {
		return ui.Rect{}
	}
	return ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

func (v *View) run() {
	var release func()
	defer func() {
		if release != nil {
			release()
		}
		v.unshadeAll()
	}()

	t := time.NewTicker(shadeEvery)
	defer t.Stop()
	var next, shaded time.Time
	for now := range t.C {
		if !shell.Get().Visible(v) {
			v.mu.Lock()
			v.running = false
			v.mu.Unlock()
			return
		}
		if fps := config.Get().Visual.MaxFPS; fps <= 0 || now.Sub(shaded) >= time.Second/time.Duration(fps)-2*time.Millisecond {
			shaded = now
			v.shade()
		}
		if now.Before(next) {
			continue
		}
		next = now.Add(frameEvery)

		cfg := config.Get()
		if slots := chosen(cfg.Idle); len(slots) > 0 {
			if release == nil {
				release = visuals.Get().Hold()
			}
			if v.painting(len(slots)) {
				shell.Get().Redraw()
				continue
			}
		} else {
			v.unshadeAll()
			if release != nil {
				release()
				release = nil
			}
		}

		v.mu.Lock()
		stale := v.shown != key(cfg, time.Now())
		v.mu.Unlock()
		if stale {
			shell.Get().Redraw()
		}
	}
}
