package speaker

import (
	"fmt"

	"github.com/ygelfand/libcountertop/pkg/audio/alsa"
)

type setting struct {
	name   string
	value  uint32
	choice string
}

func Route() error {
	m, err := alsa.OpenMixer(Card)
	if err != nil {
		return fmt.Errorf("speaker: %w", err)
	}
	defer m.Close()

	return apply(m, Get().hw().route())
}

func apply(m *alsa.Mixer, route []setting) error {
	for _, s := range route {
		var err error
		if s.choice != "" {
			err = m.SetEnum(s.name, s.choice)
		} else {
			err = m.SetInt(s.name, s.value)
		}
		if err != nil {
			return fmt.Errorf("speaker: routing %s: %w", s.name, err)
		}
	}
	return nil
}
