package speaker

import (
	"fmt"
	"sync"
	"time"

	"github.com/ygelfand/libcountertop/pkg/audio/alsa"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/mtkaudio"
)

type mediatek struct {
	chip *board.Chip

	mu     sync.Mutex
	tables mtkaudio.Tables
	loaded bool
	dB     float64
}

func newMediatek(b board.Board) *mediatek { return &mediatek{chip: b.Amp} }

func (m *mediatek) route() []setting { return nil }

func (m *mediatek) stereo() bool { return true }

func (m *mediatek) pipeline() time.Duration { return 4 * time.Millisecond }

func (m *mediatek) power(out *alsa.Playback, on bool) error {
	if m.chip == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.loaded {
		return mtkaudio.AmpPlay(*m.chip, m.tables, on)
	}
	if !on {
		return nil
	}

	t, err := mtkaudio.Stock()
	if err != nil {
		return err
	}
	stop := silence(out)
	err = mtkaudio.AmpOn(*m.chip, t)
	if serr := stop(); err == nil && serr != nil {
		err = fmt.Errorf("speaker: silence while loading the amplifier: %w", serr)
	}
	if err != nil {
		return err
	}
	m.tables, m.loaded = t, true
	return mtkaudio.AmpVolume(*m.chip, m.dB)
}

func (m *mediatek) volume(_ *alsa.Mixer, dB float64) (bool, error) {
	if m.chip == nil {
		return false, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.dB = dB
	if !m.loaded {
		return true, nil
	}
	return true, mtkaudio.AmpVolume(*m.chip, dB)
}

func silence(out *alsa.Playback) (stop func() error) {
	if out == nil {
		return func() error { return nil }
	}
	quit, done := make(chan struct{}), make(chan error, 1)
	go func() {
		quiet := make([]byte, period*FrameBytes)
		for {
			select {
			case <-quit:
				done <- nil
				return
			default:
			}
			if _, err := out.Write(quiet); err != nil {
				done <- err
				return
			}
		}
	}()
	return func() error {
		close(quit)
		return <-done
	}
}
