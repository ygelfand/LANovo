package mic

import (
	"fmt"

	"github.com/ygelfand/LANovo/internal/lib/alsa"
)

// qualcommRoute is the mixer path from the two digital microphones to the capture stream.
//
// TERT_MI2S_TX Channels is the one that matters and the one that is not in
// mixer_paths_openq624_fep.xml: the XML sets MI2S_TX Channels, which belongs to the primary MI2S,
// while capture runs on the tertiary. The HAL sets the tertiary from its own code, so replaying
// the XML alone leaves the stream mono — two channels that are bit-identical, which looks like
// working stereo and is one microphone copied.
var qualcommRoute = []setting{
	{name: "MultiMedia1 Mixer TERT_MI2S_TX", value: 1},
	{name: "DEC1 MUX", choice: "DMIC1"},
	{name: "DEC2 MUX", choice: "DMIC2"},
	{name: "MI2S_TX Channels", choice: "Two"},
	{name: "TERT_MI2S_TX Channels", choice: "Two"},
}

// gains are the digital gain on each decimator. Digital, so it lifts the noise floor with the
// signal.
var gains = []string{"DEC1 Volume", "DEC2 Volume"}

// qualcomm takes the codec's digital microphones through its decimators.
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
