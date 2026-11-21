package light

import (
	"slices"
	"testing"

	"github.com/ygelfand/libcountertop/pkg/system/i2c"
)

type byteChip struct {
	regs   [256]byte
	writes [][2]byte
}

func (c *byteChip) Close() error { return nil }

func (c *byteChip) Transfer(msgs ...i2c.Msg) error {
	switch {
	case len(msgs) == 2 && !msgs[0].Read && msgs[1].Read:
		reg := msgs[0].Buf[0]
		for i := range msgs[1].Buf {
			msgs[1].Buf[i] = c.regs[reg+byte(i)]
		}
	case len(msgs) == 1 && !msgs[0].Read && len(msgs[0].Buf) == 2:
		c.regs[msgs[0].Buf[0]] = msgs[0].Buf[1]
		c.writes = append(c.writes, [2]byte{msgs[0].Buf[0], msgs[0].Buf[1]})
	}
	return nil
}

func ltrOn(t *testing.T, c *byteChip) *LTR578 {
	t.Helper()
	dev, err := i2c.At(c, ltr578Address)
	if err != nil {
		t.Fatal(err)
	}
	s, err := AttachLTR578(dev)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestTheLTR578StartsTheWayStockDoes(t *testing.T) {
	c := &byteChip{}
	ltrOn(t, c)
	want := [][2]byte{
		{0x00, 0x01},
		{0x01, 0x36},
		{0x02, 16},
		{0x03, 0x5c},
		{0x00, 0x03},
		{0x05, 4},
		{0x04, 3},
	}
	if !slices.Equal(c.writes, want) {
		t.Errorf("wrote %x, want %x", c.writes, want)
	}
}

func TestTheLTR578IsKnownByItsPartNumber(t *testing.T) {
	for id, want := range map[byte]bool{0xb1: true, 0xb2: true, 0x00: false, 0xa1: false, 0x58: false} {
		c := &byteChip{}
		c.regs[ltrPartID] = id
		dev, err := i2c.At(c, ltr578Address)
		if err != nil {
			t.Fatal(err)
		}
		if got := isLTR578(dev); got != want {
			t.Errorf("part id %#02x: %v, want %v", id, got, want)
		}
	}
}

func TestTheLTR578ReadsLuxAsStockDoes(t *testing.T) {
	for _, tt := range []struct {
		counts int
		want   float64
	}{
		{0, 0},
		{3, 0},
		{4, 4.0 / 3},
		{300, 100},
		{0xfffff, float64(0xfffff) / 3},
	} {
		c := &byteChip{}
		s := ltrOn(t, c)
		c.regs[ltrALSData] = byte(tt.counts)
		c.regs[ltrALSData+1] = byte(tt.counts >> 8)
		c.regs[ltrALSData+2] = byte(tt.counts>>16) | 0xf0
		got, err := s.Lux()
		if err != nil || got != tt.want {
			t.Errorf("%d counts: %v lux (%v), want %v", tt.counts, got, err, tt.want)
		}
	}
}

func TestTheLTR578ReadsElevenBitProximityAndOverflow(t *testing.T) {
	for _, tt := range []struct {
		lo, hi byte
		want   int
	}{
		{0x00, 0x00, 0},
		{0x34, 0x02, 0x234},
		{0xff, 0xf7, 2047},
		{0x10, 0x08, 2047},
	} {
		c := &byteChip{}
		s := ltrOn(t, c)
		c.regs[ltrPSData], c.regs[ltrPSData+1] = tt.lo, tt.hi
		got, err := s.Proximity()
		if err != nil || got != tt.want {
			t.Errorf("%#02x %#02x: %d (%v), want %d", tt.lo, tt.hi, got, err, tt.want)
		}
	}
}
