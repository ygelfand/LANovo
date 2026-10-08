package i2c

import (
	"encoding/binary"
	"fmt"
)

type Msg struct {
	Addr uint16
	Read bool
	Buf  []byte
}

type Bus interface {
	Transfer(msgs ...Msg) error
	Close() error
}

type Device struct {
	bus  Bus
	addr uint16
}

// I2C addresses are 7-bit: anything above 0x77 is not one.
func At(bus Bus, addr uint16) (*Device, error) {
	if addr > 0x77 {
		return nil, fmt.Errorf("i2c: %#x is not a 7-bit address", addr)
	}
	return &Device{bus: bus, addr: addr}, nil
}

func (d *Device) Addr() uint16 { return d.addr }

func (d *Device) Read(reg byte, into []byte) error {
	if len(into) == 0 {
		return nil
	}
	return d.bus.Transfer(
		Msg{Addr: d.addr, Buf: []byte{reg}},
		Msg{Addr: d.addr, Read: true, Buf: into},
	)
}

func (d *Device) Write(reg byte, data ...byte) error {
	return d.bus.Transfer(Msg{Addr: d.addr, Buf: append([]byte{reg}, data...)})
}

func (d *Device) Byte(reg byte) (byte, error) {
	var b [1]byte
	if err := d.Read(reg, b[:]); err != nil {
		return 0, err
	}
	return b[0], nil
}

func (d *Device) ReadU16(reg byte) (uint16, error) {
	var b [2]byte
	if err := d.Read(reg, b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(b[:]), nil
}

func (d *Device) WriteU16(reg byte, v uint16) error {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	return d.Write(reg, b[0], b[1])
}

func WriteRows(n int, addr uint16, rows [][2]byte) error {
	bus, err := Open(n)
	if err != nil {
		return err
	}
	defer bus.Close()
	dev, err := At(bus, addr)
	if err != nil {
		return err
	}
	for i, r := range rows {
		if err := dev.Write(r[0], r[1]); err != nil {
			if err := dev.Write(r[0], r[1]); err != nil {
				return fmt.Errorf("i2c: %#02x row %d (%#02x=%#02x): %w", addr, i, r[0], r[1], err)
			}
		}
	}
	return nil
}

func Pairs(b []byte) ([][2]byte, error) {
	if len(b)%2 != 0 {
		return nil, fmt.Errorf("i2c: a table of %d bytes is not whole pairs", len(b))
	}
	rows := make([][2]byte, len(b)/2)
	for i := range rows {
		rows[i] = [2]byte{b[2*i], b[2*i+1]}
	}
	return rows, nil
}

func Probe(bus Bus, addr uint16) bool {
	var b [1]byte
	return bus.Transfer(Msg{Addr: addr, Read: true, Buf: b[:]}) == nil
}

func Scan(bus Bus) []uint16 {
	var found []uint16
	for addr := uint16(0x03); addr <= 0x77; addr++ {
		if Probe(bus, addr) {
			found = append(found, addr)
		}
	}
	return found
}

func Find(buses []int, addr uint16, identify func(*Device) bool) (*Device, Bus, error) {
	for _, n := range buses {
		bus, err := Open(n)
		if err != nil {
			continue
		}

		d, err := At(bus, addr)
		if err != nil {
			_ = bus.Close()
			return nil, nil, err
		}
		if identify(d) {
			return d, bus, nil
		}
		_ = bus.Close()
	}
	return nil, nil, fmt.Errorf(
		"i2c: nothing at %#x on any of %v answered as expected",
		addr,
		buses,
	)
}
