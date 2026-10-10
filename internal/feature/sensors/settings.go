package sensors

import (
	"log/slog"
	"strconv"
	"sync"

	"github.com/ygelfand/libcountertop/pkg/say"
	setting "github.com/ygelfand/libcountertop/pkg/settings"

	"github.com/ygelfand/LANovo/internal/config"
)

type Setting = setting.Setting[config.Presence]

const PresenceGroup setting.Group = "Presence"

var rises = [config.PresenceRangeMax + 1]float64{0, 150, 60, 18, 6, 3}

func rise(r int) float64 {
	return rises[max(config.PresenceRangeMin, min(r, config.PresenceRangeMax))]
}

var (
	table *setting.Table[config.Presence]
	built sync.Once
)

func Table() *setting.Table[config.Presence] {
	built.Do(func() {
		table = setting.NewTable("presence", []setting.Group{PresenceGroup}, []Setting{
			{
				Name:  "wake",
				Group: PresenceGroup,
				ID:    "presence_wake",
				Icon:  "mdi:account-arrow-right",
				Kind:  setting.Toggle,
				Read:  func(p *config.Presence) string { return setting.OnOff(p.Wake) },
				Write: func(p *config.Presence, v string) error {
					on, ok := setting.Boolean(v)
					if !ok {
						return Table().MustRow("wake").Bad(v, "on or off")
					}
					p.Wake = on
					return nil
				},
			},
			{
				Name:   "range",
				Group:  PresenceGroup,
				ID:     "presence_range",
				Icon:   "mdi:signal-distance-variant",
				Kind:   setting.Number,
				Slider: true,
				Min:    config.PresenceRangeMin,
				Max:    config.PresenceRangeMax,
				Read:   func(p *config.Presence) string { return strconv.Itoa(p.Range) },
				Write: func(p *config.Presence, v string) error {
					n, err := Table().MustRow("range").Number(v)
					if err != nil {
						return err
					}
					p.Range = n
					return nil
				},
			},
		}, setting.Messages{Text: say.T, Missing: say.Missing}).Local()
	})
	return table
}

func SetPresence(name, value string) error {
	p := config.Get().Presence
	row, err := Table().Row(name)
	if err != nil {
		return err
	}
	if err := row.Write(&p, value); err != nil {
		return err
	}
	if err := config.Set().Presence().Set(p); err != nil {
		return err
	}
	controls().Publish()
	return nil
}

var (
	presenceControls *setting.Controls[config.Presence]
	controlsOnce     sync.Once
)

func controls() *setting.Controls[config.Presence] {
	controlsOnce.Do(func() {
		presenceControls = &setting.Controls[config.Presence]{
			Table: Table(),
			Read:  func() config.Presence { return config.Get().Presence },
			Save: func(s Setting, v string) error {
				err := SetPresence(s.Name, v)
				if err != nil {
					slog.Warn("a presence setting was refused", "setting", s.Name, "err", err)
				}
				return err
			},
		}
	})
	return presenceControls
}
