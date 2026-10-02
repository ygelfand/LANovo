package speaker

import (
	"fmt"

	"github.com/ygelfand/LANovo/internal/lib/alsa"
)

// setting is one mixer control and what to put in it: a number, or the name of a choice for an
// enumerated one.
type setting struct {
	name   string
	value  uint32
	choice string
}

// Route puts the path in place on its own, for a tool that wants the routing without the service.
func Route() error {
	m, err := alsa.OpenMixer(Card)
	if err != nil {
		return fmt.Errorf("speaker: %w", err)
	}
	defer m.Close()

	return apply(m, Get().hw().route())
}

// apply puts the whole path in place.
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
