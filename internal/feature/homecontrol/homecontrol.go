package homecontrol

import (
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/feature/homeassistant"
	"github.com/ygelfand/LANovo/internal/feature/shell"
	"github.com/ygelfand/LANovo/internal/lib/say"
	"github.com/ygelfand/LANovo/internal/ui/widget"
)

type Selection struct {
	Key    string
	Filter homeassistant.Filter
}

var Selections = []Selection{
	{Key: "lights", Filter: homeassistant.Filter{Domains: []string{"light"}}},
	{Key: "switches", Filter: homeassistant.Filter{Domains: []string{"switch"}}},
}

func (s Selection) Name() string { return say.T("home.selection." + s.Key) }

func (s Selection) Pick() config.HomePick { return config.Get().Home.Picked(s.Key) }

func Includes(p config.HomePick, e homeassistant.Entity) bool {
	return p.All ||
		slices.Contains(p.Entities, e.ID) ||
		(e.AreaID != "" && slices.Contains(p.Areas, e.AreaID)) ||
		slices.ContainsFunc(e.Labels, func(l string) bool { return slices.Contains(p.Labels, l) })
}

func Summary(p config.HomePick) string {
	switch {
	case p.All:
		return say.T("home.all")
	case p.Empty():
		return say.T("home.none")
	}
	var parts []string
	for _, part := range []struct {
		kind string
		n    int
	}{
		{"labels", len(p.Labels)},
		{"areas", len(p.Areas)},
		{"entities", len(p.Entities)},
	} {
		if part.n > 0 {
			parts = append(parts, say.F("home.picked."+part.kind, map[string]any{"N": part.n}))
		}
	}
	return strings.Join(parts, ", ")
}

func (s Selection) Controlled() bool { return config.Get().Home.Control[s.Key] }

func (s Selection) SetControlled(on bool) {
	if err := config.Set().Home().Control(s.Key, on); err != nil {
		slog.Error("the control setting could not be saved", "selection", s.Key, "err", err)
	}
	shell.Get().Redraw()
}

func Combined() bool {
	return config.Get().Home.Combine && bothControlled()
}

func bothControlled() bool {
	for _, s := range Selections {
		if !s.Controlled() {
			return false
		}
	}
	return true
}

func SetCombined(on bool) {
	if err := config.Set().Home().Combine(on); err != nil {
		slog.Error("the combine setting could not be saved", "err", err)
	}
	shell.Get().Redraw()
}

func SettingsPage() *shell.Page {
	return &shell.Page{
		Title: say.T("home.settings"),
		Build: func() ([]widget.Row, []func(int)) {
			var (
				rows []widget.Row
				taps []func(int)
			)
			for _, s := range Selections {
				on := s.Controlled()
				rows = append(rows, widget.Row{Label: say.T("home.control." + s.Key), Kind: widget.Toggle, On: on})
				taps = append(taps, func(int) { s.SetControlled(!on) })
			}
			if bothControlled() {
				on := config.Get().Home.Combine
				rows = append(rows, widget.Row{Label: say.T("home.combine"), Kind: widget.Toggle, On: on})
				taps = append(taps, func(int) { SetCombined(!on) })
			}
			return rows, taps
		},
	}
}

type Home struct{}

var (
	home   = &Home{}
	listen sync.Once
)

func Page() *Home {
	listen.Do(func() {
		homeassistant.Get().Changed.Listen(func(homeassistant.Access) { shell.Get().Redraw() })
	})
	return home
}

func (*Home) Covers() bool { return true }

func (*Home) Timeout() time.Duration { return shell.SettingsTimeout }
