// Package i2c talks to the chips on the board's I2C buses.
//
// Lenovo's app drove the sensors through Android Things' user-driver API rather than a kernel
// driver, so with the app gone nothing reads them and the bus is ours.
package i2c

import (
	"encoding/binary"
	"fmt"
)

// Msg is one leg of a transfer: an address, a direction, and the bytes.
type Msg struct {
	Addr uint16
	Read bool
	Buf  []byte
}

// Bus carries transfers to the chips on it. A transfer of several messages is sent as one, with a
// repeated start between them, which is what reading a register takes.
type Bus interface {
	Transfer(msgs ...Msg) error
	Close() error
}

// Device is one chip at one address.
type Device struct {
	bus  Bus
	addr uint16
}

// At is the chip at this address on this bus. Addresses are 7-bit: anything above 0x77 is not one.
func At(bus Bus, addr uint16) (*Device, error) {
	if addr > 0x77 {
		return nil, fmt.Errorf("i2c: %#x is not a 7-bit address", addr)
	}
	return &Device{bus: bus, addr: addr}, nil
}

// Addr is where the chip sits.
func (d *Device) Addr() uint16 { return d.addr }

// Read fills into from a register: the register is written, then the bytes are read back without
// releasing the bus.
func (d *Device) Read(reg byte, into []byte) error {
	if len(into) == 0 {
		return nil
	}
	return d.bus.Transfer(
		Msg{Addr: d.addr, Buf: []byte{reg}},
		Msg{Addr: d.addr, Read: true, Buf: into},
	)
}

// Write sets a register.
func (d *Device) Write(reg byte, data ...byte) error {
	return d.bus.Transfer(Msg{Addr: d.addr, Buf: append([]byte{reg}, data...)})
}

// Byte is one register.
func (d *Device) Byte(reg byte) (byte, error) {
	var b [1]byte
	if err := d.Read(reg, b[:]); err != nil {
		return 0, err
	}
	return b[0], nil
}

// ReadU16 is a 16-bit register, low byte first, which is what both parts on this board use.
func (d *Device) ReadU16(reg byte) (uint16, error) {
	var b [2]byte
	if err := d.Read(reg, b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(b[:]), nil
}

// WriteU16 sets a 16-bit register, low byte first.
func (d *Device) WriteU16(reg byte, v uint16) error {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	return d.Write(reg, b[0], b[1])
}

// WriteRows opens bus n and writes each (register, value) pair to addr in order.
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

// Pairs splits a flat (register, value) byte table into rows.
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

// Probe reports whether anything answers at an address. A one byte read is the gentlest question
// that still needs an acknowledgement: a chip that is not there leaves the bus unacknowledged and
// the transfer fails.
func Probe(bus Bus, addr uint16) bool {
	var b [1]byte
	return bus.Transfer(Msg{Addr: addr, Read: true, Buf: b[:]}) == nil
}

// Scan is every address on a bus that answers.
func Scan(bus Bus) []uint16 {
	var found []uint16
	for addr := uint16(0x03); addr <= 0x77; addr++ {
		if Probe(bus, addr) {
			found = append(found, addr)
		}
	}
	return found
}

// Find looks for a chip across the buses it might be on, by asking each candidate to identify
// itself. Which kernel bus a chip sits on is not something to hardcode: the SoC's numbering and
// the kernel's do not have to agree.
//
// identify is given a device and reports whether it is the right chip, which for both parts here
// means reading an identity register.
func Find(buses []int, addr uint16, identify func(*Device) bool) (*Device, Bus, error) {
	for _, n := range buses {
		bus, err := Open(n)
		if err != nil {
			continue
		}

		d, err := At(bus, addr)
		if err != nil {
			bus.Close()
			return nil, nil, err
		}
		if identify(d) {
			return d, bus, nil
		}
		bus.Close()
	}
	return nil, nil, fmt.Errorf("i2c: nothing at %#x on any of %v answered as expected", addr, buses)
}
