package speaker

import (
	"fmt"
	"time"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/gpio"
	"github.com/ygelfand/LANovo/internal/hardware/qcomaudio"
	"github.com/ygelfand/LANovo/internal/layout"
	"github.com/ygelfand/LANovo/internal/lib/alsa"
)

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
	amp    gpio.Pin
	chip   *board.Chip
	loaded bool
}

func (q *qualcomm) route() []setting { return qualcommRoute }

func (q *qualcomm) stereo() bool { return false }

func (q *qualcomm) pipeline() time.Duration { return 48 * time.Millisecond }

func (q *qualcomm) power(out *alsa.Playback, on bool) error {
	if q.amp.N == 0 {
		pin, err := gpio.Output(layout.GPIOAmpEnable)
		if err != nil {
			return fmt.Errorf("speaker: amplifier enable: %w", err)
		}
		q.amp = pin
	}
	if err := q.amp.Set(on); err != nil {
		return err
	}
	if !on || q.loaded || q.chip == nil {
		return nil
	}

	stop := silence(out)
	err := qcomaudio.AmpOn(*q.chip)
	if serr := stop(); err == nil && serr != nil {
		err = fmt.Errorf("speaker: silence while loading the amplifier: %w", serr)
	}
	if err != nil {
		return err
	}
	q.loaded = true
	return nil
}

func (q *qualcomm) volume(*alsa.Mixer, float64) (bool, error) { return false, nil }
