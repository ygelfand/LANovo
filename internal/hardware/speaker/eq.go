package speaker

import (
	"embed"
	"fmt"
	"log/slog"
	"sync"

	esphome "github.com/ygelfand/go-esphome-device"

	"github.com/ygelfand/libcountertop/pkg/audio/hkeq"
	"github.com/ygelfand/libcountertop/pkg/say"
	setting "github.com/ygelfand/libcountertop/pkg/settings"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/component"
	"github.com/ygelfand/LANovo/internal/config"
)

//go:embed tuning
var tunings embed.FS

const Group setting.Group = "Speaker"

type eq struct {
	chains [Channels]*hkeq.Chain
	on     bool
}

func loadEQ(name string) (*eq, error) {
	raw, err := tunings.ReadFile("tuning/" + name + ".hkeq")
	if err != nil {
		return nil, fmt.Errorf("speaker: tuning %q: %w", name, err)
	}
	t, err := hkeq.Parse(raw)
	if err != nil {
		return nil, err
	}
	e := &eq{}
	for c := range e.chains {
		e.chains[c] = t.Chain()
	}
	return e, nil
}

func (e *eq) step(c int, v float32) float32 {
	return float32(e.chains[c].Step(float64(v)/32768) * 32768)
}

func (e *eq) reset() {
	for _, c := range e.chains {
		c.Reset()
	}
}

func Table() *setting.Table[config.Speaker] {
	return setting.NewTable("speaker", []setting.Group{Group}, []setting.Setting[config.Speaker]{
		setting.Flag(
			setting.Setting[config.Speaker]{
				Name:  "eq",
				Group: Group,
				ID:    "speaker_eq",
				Icon:  "mdi:equalizer",
			},
			func(s *config.Speaker) *bool { return &s.EQ },
		),
	}, setting.Messages{Text: say.T, Missing: say.Missing})
}

var (
	eqControls *setting.Controls[config.Speaker]
	eqOnce     sync.Once
)

func (s *Speaker) tuning() *eq {
	s.eqOnce.Do(func() {
		name := board.Current().SpeakerTuning
		if name == "" {
			return
		}
		e, err := loadEQ(name)
		if err != nil {
			slog.Warn("the speaker tuning could not be loaded", "tuning", name, "err", err)
			return
		}
		s.eqChains = e
		s.eqWanted.Store(config.SpeakerSection.Read().EQ)
		eqOnce.Do(func() {
			eqControls = &setting.Controls[config.Speaker]{
				Table:  Table(),
				Device: component.DevicePlayback,
				Read:   config.SpeakerSection.Read,
				Save: func(st setting.Setting[config.Speaker], v string) error {
					if err := setting.Store(config.SpeakerSection, st, v); err != nil {
						return err
					}
					s.eqWanted.Store(config.SpeakerSection.Read().EQ)
					return nil
				},
			}
			component.Settings.Add(eqControls)
		})
	})
	return s.eqChains
}

func (s *Speaker) Entities() []esphome.Entity {
	if s.tuning() == nil {
		return nil
	}
	return eqControls.Entities()
}

func (s *Speaker) Restore(c config.Config) {
	if s.tuning() == nil {
		return
	}
	s.eqWanted.Store(c.Speaker.EQ)
	eqControls.PublishFrom(c.Speaker)
}

func (s *Speaker) EQ() (available, on bool) {
	return s.tuning() != nil, s.eqWanted.Load()
}

func (s *Speaker) SetEQ(on bool) error {
	if s.tuning() == nil {
		return fmt.Errorf("speaker: this board has no tuning")
	}
	return eqControls.Change("eq", setting.OnOff(on))
}
