package mic

import (
	"errors"
	"sync"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/mtkaudio"
	"github.com/ygelfand/LANovo/internal/lib/alsa"
)

// The TLV320ADC3101 sends 24-bit samples in 32-bit I2S words.
type mediatek struct {
	chip *board.Chip

	mu     sync.Mutex
	loaded bool
}

func (*mediatek) device() int { return 1 }

func (*mediatek) bits() int { return 32 }

func (a *mediatek) open(*alsa.Mixer) error {
	if a.chip == nil {
		return errors.New("mic: this board has no microphone ADC")
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.loaded {
		return nil
	}
	t, err := mtkaudio.Stock()
	if err != nil {
		return err
	}
	if err := mtkaudio.MicOn(*a.chip, t); err != nil {
		return err
	}
	a.loaded = true
	return nil
}

func (a *mediatek) gain(_ *alsa.Mixer, gain int) error {
	if a.chip == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.loaded {
		return nil
	}
	return mtkaudio.MicGain(*a.chip, float64(gain-DefaultGain))
}
