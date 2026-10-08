package light

import (
	"errors"
	"testing"

	"github.com/ygelfand/LANovo/internal/hardware/i2c"
)

type chip struct {
	regs map[byte]uint16
	err  error
}

func newChip() *chip {
	return &chip{regs: map[byte]uint16{regDeviceID: deviceID}}
}

func (c *chip) Close() error { return nil }

func (c *chip) Transfer(msgs ...i2c.Msg) error {
	if c.err != nil {
		return c.err
	}

	switch {
	case len(msgs) == 2 && !msgs[0].Read && msgs[1].Read:
		v := c.regs[msgs[0].Buf[0]]
		msgs[1].Buf[0] = byte(v)
		if len(msgs[1].Buf) > 1 {
			msgs[1].Buf[1] = byte(v >> 8)
		}
	case len(msgs) == 1 && !msgs[0].Read && len(msgs[0].Buf) == 3:
		c.regs[msgs[0].Buf[0]] = uint16(msgs[0].Buf[1]) | uint16(msgs[0].Buf[2])<<8
	}
	return nil
}

func attached(t *testing.T, c *chip) *Sensor {
	t.Helper()

	dev, err := i2c.At(c, Address)
	if err != nil {
		t.Fatalf("At: %v", err)
	}
	s, err := Attach(dev)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	return s
}

func TestAttachStartsBothHalves(t *testing.T) {
	c := newChip()
	c.regs[regALSConf] = 0x0001
	c.regs[regPSConf12] = 0x0001

	attached(t, c)

	if got := c.regs[regALSConf]; got != running {
		t.Errorf("ambient light config is %#04x, want %#04x", got, running)
	}
	if got := c.regs[regPSConf12]; got&1 != 0 || got != psReach {
		t.Errorf("proximity config is %#04x, want %#04x", got, psReach)
	}
	if got := c.regs[regPSConf3]; got != psLED {
		t.Errorf("proximity emitter is %#04x, want %#04x", got, psLED)
	}
}

func TestIdentify(t *testing.T) {
	c := newChip()

	dev, err := i2c.At(c, Address)
	if err != nil {
		t.Fatalf("At: %v", err)
	}
	if !identify(dev) {
		t.Error("the part did not identify itself")
	}

	c.regs[regDeviceID] = 0x0000
	if identify(dev) {
		t.Error("a chip with the wrong id identified as this one")
	}
}

func TestIdentifyRejectsABusThatFails(t *testing.T) {
	c := newChip()
	c.err = errors.New("no acknowledgement")

	dev, err := i2c.At(c, Address)
	if err != nil {
		t.Fatalf("At: %v", err)
	}
	if identify(dev) {
		t.Error("a bus that failed identified as the part")
	}
}

func TestLux(t *testing.T) {
	tests := []struct {
		name   string
		counts uint16
		want   float64
	}{
		{"dark", 0, 0},
		{"one count", 1, luxPerCount * window},
		{"a lit room", 1000, 24 * window},

		{"full scale", 0xffff, float64(0xffff) * luxPerCount * window},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newChip()
			s := attached(t, c)
			c.regs[regALSData] = tt.counts

			got, err := s.Lux()
			if err != nil {
				t.Fatalf("Lux: %v", err)
			}
			if got != tt.want {
				t.Errorf("Lux() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProximity(t *testing.T) {
	c := newChip()
	s := attached(t, c)
	c.regs[regPSData] = 1234

	got, err := s.Proximity()
	if err != nil {
		t.Fatalf("Proximity: %v", err)
	}
	if got != 1234 {
		t.Errorf("Proximity() = %d, want 1234", got)
	}
}

func TestReadErrorsReachTheCaller(t *testing.T) {
	c := newChip()
	s := attached(t, c)
	c.err = errors.New("no acknowledgement")

	if _, err := s.Lux(); err == nil {
		t.Error("Lux swallowed the bus error")
	}
	if _, err := s.Proximity(); err == nil {
		t.Error("Proximity swallowed the bus error")
	}
}
