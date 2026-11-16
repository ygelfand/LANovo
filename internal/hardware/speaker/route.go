package speaker

import (
	"fmt"

	"github.com/ygelfand/libcountertop/pkg/audio/alsa"

	"github.com/ygelfand/LANovo/internal/board"
)

func Route() error {
	m, err := alsa.OpenMixer(Card)
	if err != nil {
		return fmt.Errorf("speaker: %w", err)
	}
	defer m.Close()

	return apply(m, Get().hw().route())
}

func apply(m *alsa.Mixer, route []board.Mixer) error {
	for _, s := range route {
		var err error
		if s.Choice != "" {
			err = m.SetEnum(s.Name, s.Choice)
		} else {
			err = m.SetInt(s.Name, s.Value)
		}
		if err != nil {
			return fmt.Errorf("speaker: routing %s: %w", s.Name, err)
		}
	}
	return nil
}
