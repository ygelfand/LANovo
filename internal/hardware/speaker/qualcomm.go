package speaker

import (
	"fmt"
	"time"

	"github.com/ygelfand/libcountertop/pkg/audio/alsa"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/gpio"
	"github.com/ygelfand/LANovo/internal/hardware/qcomaudio"
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

type qualcomm struct {
	wired  []int
	both   bool
	pins   []gpio.Pin
	chip   *board.Chip
	loaded bool
}

func (q *qualcomm) route() []setting { return qualcommRoute }

func (q *qualcomm) stereo() bool { return q.both }

func (q *qualcomm) pipeline() time.Duration { return 48 * time.Millisecond }

func (q *qualcomm) power(out *alsa.Playback, on bool) error {
	if len(q.pins) != len(q.wired) {
		q.pins = q.pins[:0]
		for _, n := range q.wired {
			pin, err := gpio.Output(n)
			if err != nil {
				return fmt.Errorf("speaker: amplifier pin: %w", err)
			}
			q.pins = append(q.pins, pin)
		}
	}
	for i := range q.pins {
		pin := q.pins[i]
		if !on {
			pin = q.pins[len(q.pins)-1-i]
		}
		if err := pin.Set(on); err != nil {
			return err
		}
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
