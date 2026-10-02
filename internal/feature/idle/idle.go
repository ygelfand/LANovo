package idle

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	gogui "github.com/go-gui-org/go-gui/gui"
	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/clock"
	"github.com/ygelfand/LANovo/internal/feature/drawer"
	"github.com/ygelfand/LANovo/internal/feature/sensors"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/feature/videoplayer"
	"github.com/ygelfand/LANovo/internal/feature/web"
	"github.com/ygelfand/LANovo/internal/hardware/display"
	"github.com/ygelfand/LANovo/internal/hardware/touch"
	"github.com/ygelfand/LANovo/internal/lib/say"
	"github.com/ygelfand/LANovo/internal/ui/visual"
)

const check = 200 * time.Millisecond

type Idle struct {
	after, face, place, align, size *esphome.Select
	kinds, sources                  [2]*esphome.Select

	mu   sync.Mutex
	view *View
	hold *shell.Hold
}

var (
	once   sync.Once
	shared *Idle
)

func init() {
	component.Register(component.Device, Get, component.Order(35))
}

func Get() *Idle {
	once.Do(func() {
		shared = &Idle{}
		shared.view = newView()
		shared.build()
		sensors.Get().Arrived.Listen(func(near bool) {
			if near && config.Get().Presence.Wake {
				shared.wake()
			}
		})
		drawer.Get().Add(drawer.Entry{
			Name:  func() string { return say.T("rail.sleep") },
			Order: drawer.OrderIdle,
			Glyph: func() string { return gogui.IconMoon },
			Open:  shared.Lock,
		})
	})
	return shared
}

// Lock puts the idle screen up now, clearing whatever was open so nothing sits under it to go
// back to.
func (i *Idle) Lock() {
	shell.Get().Close()
	i.Show()
}

func (i *Idle) Name() string { return "idle" }

func (i *Idle) View() *View { return i.view }

func (i *Idle) Entities() []esphome.Entity {
	return []esphome.Entity{
		i.after, i.face, i.place, i.align, i.size,
		i.kinds[0], i.sources[0], i.kinds[1], i.sources[1],
	}
}

func (i *Idle) Restore(cfg config.Config) {
	c := cfg.Idle
	i.after.Set(c.After.Label())
	i.face.Set(c.Face.Label())
	i.place.Set(c.Position.Label())
	i.align.Set(c.Align.Label())
	i.size.Set(c.Size.Label())
	for n, v := range []config.IdleVisual{c.First, c.Second} {
		i.kinds[n].Set(KindLabel(v.Kind))
		i.sources[n].Set(v.Source.Label())
	}
}

func (i *Idle) Run(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return nil
	case <-clock.Get().Ready():
	}

	t := time.NewTicker(check)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
		i.covered()
		if due(config.Get().Idle.After.After(), quiet()) && free() {
			i.Show()
		}
	}
}

// covered takes the idle screen out of the way once anything opens over it, so going back from
// that never lands on it.
func (i *Idle) covered() {
	i.mu.Lock()
	hold := i.hold
	i.mu.Unlock()
	if hold.Held() && !shell.Get().Visible(i.view) {
		hold.Release()
	}
}

func due(after, since time.Duration) bool { return after > 0 && since > after }

func quiet() time.Duration {
	since := touch.Get().Since()
	if w := videoplayer.Watched(); !w.IsZero() {
		since = min(since, time.Since(w))
	}
	if config.Get().Presence.Wake {
		if seen, ok := sensors.Get().Seen(); ok {
			since = min(since, seen)
		}
	}
	return since
}

func free() bool {
	d := display.Get()
	return !shell.Get().Open() && d.Uncovered(display.PriorityDashboard) && web.Get().Offering() == ""
}

func (i *Idle) wake() {
	i.mu.Lock()
	hold := i.hold
	i.mu.Unlock()
	if hold.Held() && shell.Get().Visible(i.view) {
		hold.Release()
	}
}

func (i *Idle) Show() {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.hold.Held() {
		return
	}
	i.hold = shell.Get().Hold(i.view)
}

func KindLabel(kind string) string {
	if kind == "" {
		return say.T("idle.none")
	}
	return visual.Kind(kind).Label()
}

func KindLabels() []string {
	out := []string{say.T("idle.none")}
	for _, k := range visual.Built() {
		out = append(out, k.Label())
	}
	return out
}

func kindByLabel(label string) (string, bool) {
	if label == say.T("idle.none") {
		return "", true
	}
	for _, k := range visual.Built() {
		if k.Label() == label {
			return string(k), true
		}
	}
	return "", false
}

func (i *Idle) redraw() {
	i.mu.Lock()
	held := i.hold.Held()
	i.mu.Unlock()
	if held {
		shell.Get().Redraw()
	}
}

func saved(what string, err error) {
	if err != nil {
		slog.Error("saving a setting failed", "setting", what, "err", err)
	}
}

func (i *Idle) SetAfter(v config.Delay) {
	i.after.Set(v.Label())
	saved("idle_after", config.Set().Idle().After(v))
}

func (i *Idle) SetFace(v config.Face) {
	i.face.Set(v.Label())
	saved("idle_face", config.Set().Idle().Face(v))
	i.redraw()
}

func (i *Idle) SetPosition(v config.Position) {
	i.place.Set(v.Label())
	saved("idle_position", config.Set().Idle().Position(v))
	i.redraw()
}

func (i *Idle) SetAlign(v config.Align) {
	i.align.Set(v.Label())
	saved("idle_align", config.Set().Idle().Align(v))
	i.redraw()
}

func (i *Idle) SetSize(v config.Size) {
	i.size.Set(v.Label())
	saved("idle_size", config.Set().Idle().Size(v))
	i.redraw()
}

func (i *Idle) visual(slot int) config.IdleVisual {
	c := config.Get().Idle
	if slot == 0 {
		return c.First
	}
	return c.Second
}

func (i *Idle) SetKind(slot int, kind string) {
	if kind != "" && !slices.Contains(visual.Built(), visual.Kind(kind)) {
		return
	}
	v := i.visual(slot)
	v.Kind = kind
	i.kinds[slot].Set(KindLabel(kind))
	saved("idle_visual", config.Set().Idle().Visual(slot, v))
	i.redraw()
}

func (i *Idle) SetSource(slot int, s config.Source) {
	v := i.visual(slot)
	v.Source = s
	i.sources[slot].Set(s.Label())
	saved("idle_visual_source", config.Set().Idle().Visual(slot, v))
	i.redraw()
}

func (i *Idle) build() {
	sel := func(id, name, icon string, options []string) *esphome.Select {
		return &esphome.Select{
			Base: esphome.Base{ObjectID: id, Name: name, Icon: icon,
				Category: esphome.CategoryConfig, DeviceID: component.DeviceScreen},
			Options: options,
		}
	}

	i.after = sel("idle_after", "Idle screen after", "mdi:timer-outline", config.Labels(config.Delays()))
	i.after.OnCommand = func(l string) {
		if v, ok := config.ByLabel(config.Delays(), l); ok {
			i.SetAfter(v)
		}
	}
	i.face = sel("idle_face", "Idle clock face", "mdi:clock-digital", config.Labels(config.IdleFaces()))
	i.face.OnCommand = func(l string) {
		if v, ok := config.ByLabel(config.IdleFaces(), l); ok {
			i.SetFace(v)
		}
	}
	i.place = sel("idle_position", "Idle clock vertical position", "mdi:align-vertical-center", config.Labels(config.Positions()))
	i.place.OnCommand = func(l string) {
		if v, ok := config.ByLabel(config.Positions(), l); ok {
			i.SetPosition(v)
		}
	}
	i.align = sel("idle_align", "Idle clock horizontal position", "mdi:align-horizontal-center", config.Labels(config.Aligns()))
	i.align.OnCommand = func(l string) {
		if v, ok := config.ByLabel(config.Aligns(), l); ok {
			i.SetAlign(v)
		}
	}
	i.size = sel("idle_size", "Idle clock size", "mdi:resize", config.Labels(config.Sizes()))
	i.size.OnCommand = func(l string) {
		if v, ok := config.ByLabel(config.Sizes(), l); ok {
			i.SetSize(v)
		}
	}

	for n, id := range []string{"1", "2"} {
		i.kinds[n] = sel("idle_visual_"+id, "Idle visual "+id, "mdi:waveform", KindLabels())
		i.kinds[n].OnCommand = func(l string) {
			if k, ok := kindByLabel(l); ok {
				i.SetKind(n, k)
			}
		}
		i.sources[n] = sel("idle_visual_"+id+"_source", "Idle visual "+id+" listens to", "mdi:microphone-settings", config.Labels(config.Sources()))
		i.sources[n].OnCommand = func(l string) {
			if s, ok := config.ByLabel(config.Sources(), l); ok {
				i.SetSource(n, s)
			}
		}
	}
}
