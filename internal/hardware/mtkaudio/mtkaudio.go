// Package mtkaudio brings up the codec parts the stock firmware drives from its OEM app: a TAS5805M
// amplifier and a TLV320 microphone ADC, programmed over I2C with the register tables that app
// carries.
package mtkaudio

import (
	"embed"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ygelfand/LANovo/internal/board"
	"github.com/ygelfand/LANovo/internal/hardware/gpio"
	"github.com/ygelfand/LANovo/internal/hardware/i2c"
)

//go:embed tables/*.bin
var files embed.FS

// Rows are register writes, in order.
type Rows [][2]byte

// Tables are the OEM app's register tables.
type Tables struct {
	AmpInit, AmpHiZ, AmpPlay, AmpSleep, AmpMute, MicInit Rows
}

// Stock is the embedded tables, parsed once.
var Stock = sync.OnceValues(load)

func load() (Tables, error) {
	var t Tables
	for name, into := range map[string]*Rows{
		"amp_init":  &t.AmpInit,
		"amp_hiz":   &t.AmpHiZ,
		"amp_play":  &t.AmpPlay,
		"amp_sleep": &t.AmpSleep,
		"amp_mute":  &t.AmpMute,
		"mic_init":  &t.MicInit,
	} {
		b, err := files.ReadFile("tables/" + name + ".bin")
		if err != nil {
			return Tables{}, fmt.Errorf("mtkaudio: %w", err)
		}
		rows, err := i2c.Pairs(b)
		if err != nil {
			return Tables{}, fmt.Errorf("mtkaudio: %s: %w", name, err)
		}
		*into = rows
	}
	return t, nil
}

func write(c board.Chip, rows Rows) error { return i2c.WriteRows(c.Bus, c.Addr, rows) }

func drive(n int, on bool) error {
	if n == 0 {
		return nil
	}
	p, err := gpio.Output(n)
	if err != nil {
		return err
	}
	return p.Set(on)
}

// AmpOn powers the amplifier and loads it. The I2S clocks have to be running while it loads.
func AmpOn(c board.Chip, t Tables) error {
	if err := drive(c.Enable, true); err != nil {
		return fmt.Errorf("mtkaudio: amplifier enable: %w", err)
	}
	time.Sleep(100 * time.Millisecond)
	if err := write(c, t.AmpHiZ); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	return write(c, t.AmpInit)
}

// AmpPlay takes the amplifier out of deep sleep, or puts it back.
func AmpPlay(c board.Chip, t Tables, on bool) error {
	if on {
		return write(c, t.AmpPlay)
	}
	return write(c, t.AmpSleep)
}

// AmpTop is the loudest digital volume stock ever set: -11.5 dB, at 0.5 dB a step from 0x30 = 0 dB.
const AmpTop = 0x47

// AmpVolume sets the digital volume to dB below AmpTop; negative infinity mutes.
func AmpVolume(c board.Chip, dB float64) error {
	reg := byte(0xff)
	if !math.IsInf(dB, -1) {
		reg = byte(min(max(AmpTop+math.Round(-2*min(dB, 0)), AmpTop), 0xfe))
	}
	return write(c, Rows{{0x00, 0x00}, {0x7f, 0x00}, {0x4c, reg}})
}

// MicOn powers the microphone ADC, resets it and loads it.
func MicOn(c board.Chip, t Tables) error {
	if err := drive(c.Enable, true); err != nil {
		return fmt.Errorf("mtkaudio: ADC supply: %w", err)
	}
	if err := drive(c.Reset, false); err != nil {
		return fmt.Errorf("mtkaudio: ADC reset: %w", err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := drive(c.Reset, true); err != nil {
		return fmt.Errorf("mtkaudio: ADC reset: %w", err)
	}
	time.Sleep(20 * time.Millisecond)
	return write(c, t.MicInit)
}

// MicStock is the ADC digital volume the stock table ends on: +16 dB, at 0.5 dB a step.
const MicStock = 0x20

// MicGain sets both ADC channels dB above or below MicStock, within the part's -12 to +20 dB.
func MicGain(c board.Chip, dB float64) error {
	reg := byte(int8(min(max(MicStock+math.Round(2*dB), -24), 40)) & 0x7f)
	return write(c, Rows{{0x00, 0x00}, {0x53, reg}, {0x54, reg}})
}
