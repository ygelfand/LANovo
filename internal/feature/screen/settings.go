package screen

import (
	"slices"
	"strconv"
	"sync"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/setting"
	"github.com/ygelfand/LANovo/internal/ui/theme"
	"github.com/ygelfand/libcountertop/pkg/display/style"
)

// The panel's settings as a table, the way internal/feature/livecam has one.
//
// A row writes into config.Screen, which is the stored form — so unlike the camera's, where the
// table describes a capture and something else saves it, saving here is writing the field the row
// already wrote.

type (
	Setting = setting.Setting[config.Screen]
	Group   = setting.Group
)

const Panel Group = "Panel"

// Table is what Home Assistant is offered. The panel's own pages are not built from this: a theme
// is picked off a grid of swatches and a clock face off drawings of it, neither of which is a row.
var (
	table *setting.Table[config.Screen]
	built sync.Once
)

func Table() *setting.Table[config.Screen] {
	built.Do(func() {
		table = setting.NewTable("screen", []Group{Panel}, rows())
	})
	return table
}

func rows() []Setting {
	return []Setting{
		{
			Name:  "backlight",
			Group: Panel,
			ID:    "backlight", Icon: "mdi:brightness-6",
			Kind: setting.Number, Min: 0, Max: 100, Unit: "%", Slider: true,
			Read: func(s *config.Screen) string { return strconv.Itoa(s.Backlight) },
			Write: func(s *config.Screen, v string) error {
				n, err := Table().Row("backlight").Number(v)
				if err != nil {
					return err
				}
				s.Backlight = n
				return nil
			},
		},
		{
			Name:  "auto",
			Group: Panel,
			ID:    "screen_mode",
			Icon:  "mdi:brightness-auto",
			Kind:  setting.Toggle,
			Read:  func(s *config.Screen) string { return setting.OnOff(s.Mode == config.ModeAuto) },
			Write: func(s *config.Screen, v string) error {
				on, ok := setting.Boolean(v)
				if !ok {
					return Table().Row("auto").Bad(v, "on or off")
				}
				s.Mode = config.ModeManual
				if on {
					s.Mode = config.ModeAuto
				}
				return nil
			},
		},
		{
			Name:  "style",
			Group: Panel,
			ID:    "screen_style", Icon: "mdi:shape-outline",
			Kind: setting.Choice, Options: plain(style.Names()),
			Read: func(s *config.Screen) string {
				if s.Style == "" {
					return style.Standard
				}
				return s.Style
			},
			Write: func(s *config.Screen, v string) error {
				if !slices.Contains(style.Names(), v) {
					return Table().Row("style").Bad(v, "")
				}
				s.Style = v
				return nil
			},
		},
		{
			Name:  "size",
			Group: Panel,
			ID:    "screen_size", Icon: "mdi:resize",
			Kind: setting.Choice, Options: plain(config.ScreenSizes()),
			Read: func(s *config.Screen) string {
				if s.Size == "" {
					return board.Current().UISize
				}
				return s.Size
			},
			Write: func(s *config.Screen, v string) error {
				if !slices.Contains(config.ScreenSizes(), v) {
					return Table().Row("size").Bad(v, "")
				}
				s.Size = v
				return nil
			},
		},
		{
			Name:  "theme",
			Group: Panel,
			ID:    "theme", Icon: "mdi:palette",
			Kind: setting.Choice, Options: themes(),
			Read: func(s *config.Screen) string { return s.Theme },
			Write: func(s *config.Screen, v string) error {
				if _, ok := theme.ByName(v); !ok && v != style.ThemeDefault {
					return Table().Row("theme").Bad(v, "")
				}
				s.Theme = v
				return nil
			},
		},
		{
			Name:  "drawer",
			Group: Panel,
			ID:    "drawer_edge", Icon: "mdi:gesture-swipe-horizontal",
			Kind: setting.Choice, Options: edges(),
			Read: func(s *config.Screen) string { return string(s.Drawer) },
			Write: func(s *config.Screen, v string) error {
				for _, e := range config.Edges() {
					if string(e) == v {
						s.Drawer = e
						return nil
					}
				}
				return Table().Row("drawer").Bad(v, "")
			},
		},
	}
}

// A theme and an edge carry their own label, so neither needs a catalogue key of its own.
func plain(values []string) []setting.Option {
	out := make([]setting.Option, 0, len(values))
	for _, v := range values {
		out = append(out, setting.Option{Value: v, Label: v})
	}
	return out
}

func themes() []setting.Option {
	out := []setting.Option{{Value: style.ThemeDefault, Label: style.ThemeDefault}}
	for _, name := range theme.Names() {
		out = append(out, setting.Option{Value: name, Label: name})
	}
	return out
}

func edges() []setting.Option {
	out := make([]setting.Option, 0, len(config.Edges()))
	for _, e := range config.Edges() {
		out = append(out, setting.Option{Value: string(e), Label: e.Label()})
	}
	return out
}
