package motion

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/ygelfand/LANovo/internal/hardware/i2c"
)

// The address depends on how SDO is strapped.
var Addresses = []uint16{0x18, 0x19}

// i2c-2 carries the speaker amplifiers and i2c-3 the touchscreen, both bound to kernel drivers.
var buses = []int{1, 0, 2, 3}

const (
	regChipID = 0x00
	regAccX   = 0x02
	regRange  = 0x0f
	regBW     = 0x10
)

// Android names the part BMA220 and the driver class BMA253.
var chipIDs = []byte{0xfa, 0xf8, 0xfb, 0xdd}

// 12 bits: at the ±2g range, 1024 counts per g.
const (
	range2g       = 0x03
	bandwidth62Hz = 0x0c

	countsPerG = 1024.0
)

type Sensor struct{ dev *i2c.Device }

type Reading struct{ X, Y, Z float64 }

func Open() (*Sensor, i2c.Bus, error) {
	for _, addr := range Addresses {
		dev, bus, err := i2c.Find(buses, addr, identify)
		if err != nil {
			continue
		}

		s, err := Attach(dev)
		if err != nil {
			_ = bus.Close()
			return nil, nil, err
		}
		return s, bus, nil
	}
	return nil, nil, fmt.Errorf("motion: no accelerometer at %#x on any of %v", Addresses, buses)
}

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

// A signed 12-bit value in the top of a 16-bit little-endian pair.
func axis(b []byte) float64 {
	raw := int16(binary.LittleEndian.Uint16(b)) >> 4
	return float64(raw) / countsPerG
}

func (r Reading) Magnitude() float64 {
	return math.Sqrt(r.X*r.X + r.Y*r.Y + r.Z*r.Z)
}
