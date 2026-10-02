package speaker

import (
	"fmt"
	"time"

	"github.com/ygelfand/LANovo/internal/hardware/gpio"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/lib/alsa"
)

// qualcommRoute is the mixer path from the playback stream to the speaker.
//
// QUAT_MI2S_RX because that is what was set the last time this device made a sound. Routing
// MultiMedia1 to PRI_MI2S_RX instead powers the internal codec's whole playback chain in DAPM
// where QUAT powers none of it, and the device tree points the same way — qcom,msm-ext-pa is
// "primary", the protected path's feedback mux is PRI_MI2S_RX_VI_FB_MUX — but all of that is
// reading how the board is wired, and it was tried and made no sound. Observation wins.
//
// The rest is mixer_paths_openq624_fep.xml's wsa-speaker in the names this card has. None of it is
// verified either: the amplifiers do not answer on i2c at boot, so the card registers without them
// and there is nothing on the end of this path to hear. See #104.
var qualcommRoute = []setting{
	{name: "QUAT_MI2S_RX Audio Mixer MultiMedia1", value: 1},
	{name: "RX3 MIX1 INP1", choice: "RX1"},
	{name: "RX3 Digital Volume", value: rxUnity},
	{name: "LINE_OUT", choice: "Switch"},
	{name: "WSA Spk Switch", choice: "WSA"},
}

// RX digital volume 84 is 0 dB.
const rxUnity = 84

// qualcomm drives the codec's amplifier through its enable line.
type qualcomm struct {
	amp gpio.Pin
}

func (q *qualcomm) route() []setting { return qualcommRoute }

func (q *qualcomm) stereo() bool { return false }

func (q *qualcomm) pipeline() time.Duration { return 48 * time.Millisecond }

func (q *qualcomm) power(_ *alsa.Playback, on bool) error {
	if q.amp.N == 0 {
		pin, err := gpio.Output(layout.GPIOAmpEnable)
		if err != nil {
			return fmt.Errorf("speaker: amplifier enable: %w", err)
		}
		q.amp = pin
	}
	return q.amp.Set(on)
}

func (q *qualcomm) volume(*alsa.Mixer, float64) (bool, error) { return false, nil }
