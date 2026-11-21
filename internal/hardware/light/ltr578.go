package light

import (
	"fmt"

	"github.com/ygelfand/libcountertop/pkg/system/i2c"
)

const ltr578Address = 0x53

var ltr578Buses = []int{0, 1}

const (
	ltrMainCtrl   = 0x00
	ltrPSLED      = 0x01
	ltrPSPulses   = 0x02
	ltrPSMeasRate = 0x03
	ltrALSMeas    = 0x04
	ltrALSGain    = 0x05
	ltrPartID     = 0x06
	ltrPSData     = 0x08
	ltrALSData    = 0x0d
)

const (
	ltrPartNumber = 0xb0
	ltrPartMask   = 0xf0
)

const (
	ltrPSOn  = 0x01
	ltrALSOn = 0x02
)

// Stock OEM app settings: LED 60 kHz at 100 mA, 16 pulses, 11-bit every 50 ms; ALS gain 18, 20-bit every 500 ms.
const (
	ltrLED      = 0x36
	ltrPulses   = 16
	ltrPSRate   = 0x5c
	ltrGain     = 4
	ltrALSRate  = 0x03
	ltrDark     = 3
	ltrPerLux   = 3
	ltrOverflow = 0x08
	ltrPSMax    = 2047
)

var ltr578Rises = Rises{45, 30, 20, 14, 10}

type LTR578 struct{ dev *i2c.Device }

func AttachLTR578(dev *i2c.Device) (*LTR578, error) {
	s := &LTR578{dev: dev}
	for _, w := range [][2]byte{
		{ltrMainCtrl, ltrPSOn},
		{ltrPSLED, ltrLED},
		{ltrPSPulses, ltrPulses},
		{ltrPSMeasRate, ltrPSRate},
		{ltrMainCtrl, ltrPSOn | ltrALSOn},
		{ltrALSGain, ltrGain},
		{ltrALSMeas, ltrALSRate},
	} {
		if err := dev.Write(w[0], w[1]); err != nil {
			return nil, fmt.Errorf("light: starting the LTR-578: %w", err)
		}
	}
	return s, nil
}

func (s *LTR578) Emitter(pulses, led byte) error {
	if err := s.dev.Write(ltrPSPulses, pulses); err != nil {
		return fmt.Errorf("light: setting proximity pulses: %w", err)
	}
	if err := s.dev.Write(ltrPSLED, led); err != nil {
		return fmt.Errorf("light: setting the proximity emitter: %w", err)
	}
	return nil
}

func isLTR578(d *i2c.Device) bool {
	var id [1]byte
	return d.Read(ltrPartID, id[:]) == nil && id[0]&ltrPartMask == ltrPartNumber
}

func (s *LTR578) Lux() (float64, error) {
	var b [3]byte
	if err := s.dev.Read(ltrALSData, b[:]); err != nil {
		return 0, fmt.Errorf("light: reading the ambient light: %w", err)
	}
	counts := int(b[0]) | int(b[1])<<8 | int(b[2]&0x0f)<<16
	if counts <= ltrDark {
		return 0, nil
	}
	return float64(counts) / ltrPerLux, nil
}

func (s *LTR578) Proximity() (int, error) {
	var b [2]byte
	if err := s.dev.Read(ltrPSData, b[:]); err != nil {
		return 0, fmt.Errorf("light: reading proximity: %w", err)
	}
	if b[1]&ltrOverflow != 0 {
		return ltrPSMax, nil
	}
	return int(b[1]&0x07)<<8 | int(b[0]), nil
}

func (s *LTR578) Rises() Rises { return ltr578Rises }
