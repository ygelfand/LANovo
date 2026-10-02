// Package light reads the ambient light and proximity sensor.
//
// A VCNL4200, which Lenovo's app drove as an Android Things user driver rather than through a
// kernel driver, so nothing else has it.
package light

import (
	"fmt"

	"github.com/ygelfand/LANovo/internal/hardware/i2c"
)

// Address is where the part sits. Which kernel bus that is depends on the numbering, so it is
// looked for rather than assumed.
const Address = 0x51

// buses are the ones to look on. i2c-2 carries the speaker amplifiers and i2c-3 the touchscreen,
// both already bound to kernel drivers.
var buses = []int{1, 0, 2, 3}

// The registers used here. Every one is 16 bits, low byte first.
const (
	regALSConf  = 0x00
	regPSConf12 = 0x03
	regPSConf3  = 0x04
	regPSData   = 0x08
	regALSData  = 0x09
	regDeviceID = 0x0e
)

// deviceID is what the part answers with.
const deviceID = 0x1058

// Both halves come up shut down, held by bit 0 of their configuration. Clearing the register
// starts them and leaves the ambient light integration time at its shortest, 50 ms, which is bits
// 6 and 7 at zero. That is the coarsest of the four settings and the widest range: 0.024 lux a
// count, to 1573 lux, against 0.003 and 197 lux at the longest.
const (
	running     = 0x0000
	luxPerCount = 0.024
)

// PS_IT 8T, PS_HD 16-bit, LED_I 100 mA, PS_SPO set: saturation reads 0xFFFF.
const (
	psReach = 0x080e
	psLED   = 0x0a00
)

// window is what the glass over the sensor costs.
//
// The part reports the light reaching its die, and the bezel in front of it passes a fraction of
// the room — deliberately, since these windows are darkened to hide the sensor. Measured against a
// phone light meter: 188 lux in the room where the part read 5.5. Rounded, because the reference is
// a phone and worth no more than two figures.
//
// It also explains why nothing saturates indoors. The part stops at 1573 lux, but through this
// glass that is around 53000 lux of room, which is daylight.
const window = 35

// Sensor is the part.
type Sensor struct{ dev *i2c.Device }

// Open finds the sensor on whichever bus it is on and starts it measuring.
func Open() (*Sensor, i2c.Bus, error) {
	dev, bus, err := i2c.Find(buses, Address, identify)
	if err != nil {
		return nil, nil, fmt.Errorf("light: %w", err)
	}

	s, err := Attach(dev)
	if err != nil {
		bus.Close()
		return nil, nil, err
	}
	return s, bus, nil
}

// Attach starts a sensor already found on a bus.
func Attach(dev *i2c.Device) (*Sensor, error) {
	s := &Sensor{dev: dev}

	if err := s.dev.WriteU16(regALSConf, running); err != nil {
		return nil, fmt.Errorf("light: starting the ambient light sensor: %w", err)
	}
	if err := s.dev.WriteU16(regPSConf3, psLED); err != nil {
		return nil, fmt.Errorf("light: setting the proximity emitter: %w", err)
	}
	if err := s.dev.WriteU16(regPSConf12, psReach); err != nil {
		return nil, fmt.Errorf("light: starting the proximity sensor: %w", err)
	}
	return s, nil
}

// identify reads the part's own id, so the right chip is found rather than whatever answers at
// this address.
func identify(d *i2c.Device) bool {
	id, err := d.ReadU16(regDeviceID)
	return err == nil && id == deviceID
}

// Lux is the ambient light in the room, which is what the part sees through the glass corrected
// for what the glass takes.
func (s *Sensor) Lux() (float64, error) {
	counts, err := s.dev.ReadU16(regALSData)
	if err != nil {
		return 0, fmt.Errorf("light: reading the ambient light: %w", err)
	}
	return float64(counts) * luxPerCount * window, nil
}

// Proximity is how close something is, in the part's own counts: larger is nearer. It has no
// distance to report, only more or less reflected light.
func (s *Sensor) Proximity() (int, error) {
	counts, err := s.dev.ReadU16(regPSData)
	if err != nil {
		return 0, fmt.Errorf("light: reading proximity: %w", err)
	}
	return int(counts), nil
}
