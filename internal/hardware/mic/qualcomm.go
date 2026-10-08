package mic

import (
	"fmt"

	"github.com/ygelfand/LANovo/internal/lib/alsa"
)

// The HAL sets TERT_MI2S_TX Channels in code; mixer_paths_openq624_fep.xml leaves capture mono.
var qualcommRoute = []setting{
	{name: "MultiMedia1 Mixer TERT_MI2S_TX", value: 1},
	{name: "DEC1 MUX", choice: "DMIC1"},
	{name: "DEC2 MUX", choice: "DMIC2"},
	{name: "MI2S_TX Channels", choice: "Two"},
	{name: "TERT_MI2S_TX Channels", choice: "Two"},
}

var gains = []string{"DEC1 Volume", "DEC2 Volume"}

type qualcomm struct{}

func (qualcomm) device() int { return 0 }

func (qualcomm) bits() int { return 16 }

func (qualcomm) open(m *alsa.Mixer) error {
	for _, s := range qualcommRoute {
		var err error
		if s.choice != "" {
			err = m.SetEnum(s.name, s.choice)
		} else {
			err = m.SetInt(s.name, s.value)
		}
		if err != nil {
			return fmt.Errorf("mic: routing %s: %w", s.name, err)
		}
	}
	return nil
}

func (qualcomm) gain(m *alsa.Mixer, gain int) error {
	for _, name := range gains {
		if err := m.SetInt(name, uint32(gain)); err != nil {
			return fmt.Errorf("mic: %s = %d: %w", name, gain, err)
		}
	}
	return nil
}
