// Package motion reads the accelerometer.
//
// A Bosch BMA253, which Lenovo's app drove as an Android Things user driver. There is no
// gyroscope on this board: which way up the device is comes from gravity.
package motion

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/ygelfand/LANovo/internal/hardware/i2c"
)

// Addresses are where the part answers, which depends on how SDO is strapped.
var Addresses = []uint16{0x18, 0x19}

// buses are the ones to look on. i2c-2 carries the speaker amplifiers and i2c-3 the touchscreen,
// both already bound to kernel drivers.
var buses = []int{1, 0, 2, 3}

const (
	regChipID = 0x00
	regAccX   = 0x02
	regRange  = 0x0f
	regBW     = 0x10
)

// chipIDs are what this family answers with. Android named the part BMA220 and the driver class
// said BMA253, so both are accepted rather than picking a side.
var chipIDs = []byte{0xfa, 0xf8, 0xfb, 0xdd}

// The part measures 12 bits. At the ±2g range that is 1024 counts per g, which is the finest
// resolution and plenty for deciding which way is down.
const (
	range2g       = 0x03
	bandwidth62Hz = 0x0c

	countsPerG = 1024.0
)

// Sensor is the part.
type Sensor struct{ dev *i2c.Device }

// Reading is acceleration along each axis, in g.
type Reading struct{ X, Y, Z float64 }

// Open finds the accelerometer on whichever bus and address it is on.
func Open() (*Sensor, i2c.Bus, error) {
	for _, addr := range Addresses {
		dev, bus, err := i2c.Find(buses, addr, identify)
		if err != nil {
			continue
		}

		s, err := Attach(dev)
		if err != nil {
			bus.Close()
			return nil, nil, err
		}
		return s, bus, nil
	}
	return nil, nil, fmt.Errorf("motion: no accelerometer at %#x on any of %v", Addresses, buses)
}

// Attach configures a part already found on a bus.
func Attach(dev *i2c.Device) (*Sensor, error) {
	s := &Sensor{dev: dev}

	if err := s.dev.Write(regRange, range2g); err != nil {
		return nil, fmt.Errorf("motion: setting the range: %w", err)
	}
	if err := s.dev.Write(regBW, bandwidth62Hz); err != nil {
		return nil, fmt.Errorf("motion: setting the bandwidth: %w", err)
	}
	return s, nil
}

// identify reads the part's own id.
func identify(d *i2c.Device) bool {
	id, err := d.Byte(regChipID)
	if err != nil {
		return false
	}
	for _, want := range chipIDs {
		if id == want {
			return true
		}
	}
	return false
}

// Read is the current acceleration. The six data registers are read in one go, so all three axes
// come from the same moment.
func (s *Sensor) Read() (Reading, error) {
	var b [6]byte
	if err := s.dev.Read(regAccX, b[:]); err != nil {
		return Reading{}, fmt.Errorf("motion: reading acceleration: %w", err)
	}

	return Reading{
		X: axis(b[0:2]),
		Y: axis(b[2:4]),
		Z: axis(b[4:6]),
	}, nil
}

// axis turns one pair of registers into g. The part packs a signed 12-bit value into the top of a
// 16-bit little-endian pair, so it is shifted down and sign extended.
func axis(b []byte) float64 {
	raw := int16(binary.LittleEndian.Uint16(b)) >> 4
	return float64(raw) / countsPerG
}

// Magnitude is the total acceleration, which is 1g when the device is still. Anything far from
// that is being moved, and is not a reading to judge orientation from.
func (r Reading) Magnitude() float64 {
	return math.Sqrt(r.X*r.X + r.Y*r.Y + r.Z*r.Z)
}
