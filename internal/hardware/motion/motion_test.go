package motion

import (
	"errors"
	"math"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/display/geometry"

	"github.com/ygelfand/LANovo/internal/hardware/i2c"
)

type chip struct {
	regs map[byte]byte
	err  error
}

func newChip() *chip {
	return &chip{regs: map[byte]byte{regChipID: 0xfa}}
}

func (c *chip) Close() error { return nil }

func (c *chip) Transfer(msgs ...i2c.Msg) error {
	if c.err != nil {
		return c.err
	}

	switch {
	case len(msgs) == 2 && !msgs[0].Read && msgs[1].Read:
		reg := msgs[0].Buf[0]
		for i := range msgs[1].Buf {
			msgs[1].Buf[i] = c.regs[reg+byte(i)]
		}
	case len(msgs) == 1 && !msgs[0].Read:
		reg := msgs[0].Buf[0]
		for i, v := range msgs[0].Buf[1:] {
			c.regs[reg+byte(i)] = v
		}
	}
	return nil
}

func (c *chip) axis(reg byte, g float64) {
	raw := int16(math.Round(g*countsPerG)) << 4
	c.regs[reg] = byte(uint16(raw))
	c.regs[reg+1] = byte(uint16(raw) >> 8)
}

func (c *chip) at(x, y, z float64) {
	c.axis(regAccX, x)
	c.axis(regAccX+2, y)
	c.axis(regAccX+4, z)
}

func attached(t *testing.T, c *chip) *Sensor {
	t.Helper()

	dev, err := i2c.At(c, Addresses[0])
	if err != nil {
		t.Fatalf("At: %v", err)
	}
	s, err := Attach(dev)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	return s
}

func TestAttachSetsRangeAndBandwidth(t *testing.T) {
	c := newChip()
	attached(t, c)

	if got := c.regs[regRange]; got != range2g {
		t.Errorf("range is %#02x, want %#02x", got, range2g)
	}
	if got := c.regs[regBW]; got != bandwidth62Hz {
		t.Errorf("bandwidth is %#02x, want %#02x", got, bandwidth62Hz)
	}
}

func TestIdentifyAcceptsTheFamily(t *testing.T) {
	for _, id := range chipIDs {
		c := newChip()
		c.regs[regChipID] = id

		dev, err := i2c.At(c, Addresses[0])
		if err != nil {
			t.Fatalf("At: %v", err)
		}
		if !identify(dev) {
			t.Errorf("chip id %#02x was not recognized", id)
		}
	}

	c := newChip()
	c.regs[regChipID] = 0x00

	dev, _ := i2c.At(c, Addresses[0])
	if identify(dev) {
		t.Error("an unknown chip id identified as this part")
	}
}

func TestReadDecodesSignedTwelveBits(t *testing.T) {
	tests := []struct {
		name    string
		x, y, z float64
	}{
		{"stood up, gravity down the panel", 0, -1, 0},
		{"lying face up", 0, 0, 1},
		{"on its left edge", -1, 0, 0},
		{"on its right edge", 1, 0, 0},
		{"still, at rest", 0, 0, 0},
		{"a fraction", 0.5, -0.25, 0.125},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newChip()
			s := attached(t, c)
			c.at(tt.x, tt.y, tt.z)

			got, err := s.Read()
			if err != nil {
				t.Fatalf("Read: %v", err)
			}

			const tolerance = 1.0 / countsPerG
			if math.Abs(got.X-tt.x) > tolerance ||
				math.Abs(got.Y-tt.y) > tolerance ||
				math.Abs(got.Z-tt.z) > tolerance {
				t.Errorf("Read() = %+v, want %g,%g,%g", got, tt.x, tt.y, tt.z)
			}
		})
	}
}

func TestReadErrorsReachTheCaller(t *testing.T) {
	c := newChip()
	s := attached(t, c)
	c.err = errors.New("no acknowledgement")

	if _, err := s.Read(); err == nil {
		t.Error("Read swallowed the bus error")
	}
}

func TestMagnitude(t *testing.T) {
	if got := (Reading{X: 0, Y: -1, Z: 0}).Magnitude(); math.Abs(got-1) > 0.001 {
		t.Errorf("Magnitude() = %v, want 1", got)
	}
	if got := (Reading{X: 3, Y: 4, Z: 0}).Magnitude(); math.Abs(got-5) > 0.001 {
		t.Errorf("Magnitude() = %v, want 5", got)
	}
}

func TestTrackerFollowsGravity(t *testing.T) {
	tests := []struct {
		name    string
		reading Reading
		want    geometry.Orientation
	}{
		{"stood as it ships", Reading{X: 1}, geometry.Rotate90},
		{"turned to portrait", Reading{Y: -1}, geometry.Rotate0},
		{"turned the other way", Reading{X: -1}, geometry.Rotate270},
		{"upside down", Reading{Y: 1}, geometry.Rotate180},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTracker()
			tr.Update(tt.reading)

			if got := tr.Orientation(); got != tt.want {
				t.Errorf("orientation = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrackerIgnoresMovement(t *testing.T) {
	tr := NewTracker()
	tr.Update(Reading{X: 1})

	if changed := tr.Update(Reading{Y: 3}); changed {
		t.Error("a reading taken mid-movement turned the screen")
	}
	if got := tr.Orientation(); got != geometry.Rotate90 {
		t.Errorf("orientation = %v, want it held at %v", got, geometry.Rotate90)
	}
}

func TestTrackerIgnoresLyingFlat(t *testing.T) {
	tr := NewTracker()
	tr.Update(Reading{X: 1})

	if changed := tr.Update(Reading{Z: 1}); changed {
		t.Error("a device lying flat turned the screen")
	}
	if got := tr.Orientation(); got != geometry.Rotate90 {
		t.Errorf("orientation = %v, want it held at %v", got, geometry.Rotate90)
	}
}

func TestTrackerHoldsNearABoundary(t *testing.T) {
	tr := NewTracker()
	tr.Update(Reading{X: 1})

	if changed := tr.Update(Reading{X: 0.70, Y: -0.71}); changed {
		t.Error("a reading on the boundary turned the screen")
	}
	if got := tr.Orientation(); got != geometry.Rotate90 {
		t.Errorf("orientation = %v, want it held at %v", got, geometry.Rotate90)
	}
}

func TestTrackerTurnsWhenCommitted(t *testing.T) {
	tr := NewTracker()
	tr.Update(Reading{X: 1})

	if changed := tr.Update(Reading{Y: -1}); !changed {
		t.Fatal("a device turned to portrait did not follow")
	}
	if got := tr.Orientation(); got != geometry.Rotate0 {
		t.Errorf("orientation = %v, want %v", got, geometry.Rotate0)
	}
}

func TestTrackerReportsOnlyRealChanges(t *testing.T) {
	tr := NewTracker()

	if changed := tr.Update(Reading{X: 1}); changed {
		t.Error("the first reading matching how it ships reported a change")
	}
	if changed := tr.Update(Reading{X: 1}); changed {
		t.Error("an unchanged reading reported a change")
	}
}
