package light

import (
	"fmt"

	"github.com/ygelfand/libcountertop/pkg/system/i2c"
)

const vcnl4200Address = 0x51

// i2c-2 carries the speaker amplifiers and i2c-3 the touchscreen, both bound to kernel drivers.
var vcnl4200Buses = []int{1, 0, 2, 3}

// Every register is 16 bits, low byte first.
const (
	regALSConf  = 0x00
	regPSConf12 = 0x03
	regPSConf3  = 0x04
	regPSData   = 0x08
	regALSData  = 0x09
	regDeviceID = 0x0e
)

const deviceID = 0x1058

// ALS_IT 50 ms: 0.024 lux a count, 1573 lux full scale.
const (
	running     = 0x0000
	luxPerCount = 0.024
)

// PS_IT 8T, PS_HD 16-bit, LED_I 100 mA, PS_SPO set: saturation reads 0xFFFF.
const (
	psReach = 0x080e
	psLED   = 0x0a00
)

const window = 35

var vcnl4200Rises = Rises{150, 60, 18, 6, 3}

type VCNL4200 struct{ dev *i2c.Device }

func AttachVCNL4200(dev *i2c.Device) (*VCNL4200, error) {
	s := &VCNL4200{dev: dev}

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

func isVCNL4200(d *i2c.Device) bool {
	id, err := d.ReadU16(regDeviceID)
	return err == nil && id == deviceID
}

func (s *VCNL4200) Lux() (float64, error) {
	counts, err := s.dev.ReadU16(regALSData)
	if err != nil {
		return 0, fmt.Errorf("light: reading the ambient light: %w", err)
	}
	return float64(counts) * luxPerCount * window, nil
}

func (s *VCNL4200) Proximity() (int, error) {
	counts, err := s.dev.ReadU16(regPSData)
	if err != nil {
		return 0, fmt.Errorf("light: reading proximity: %w", err)
	}
	return int(counts), nil
}

func (s *VCNL4200) Rises() Rises { return vcnl4200Rises }
