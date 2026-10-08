package i2c

import (
	"errors"
	"testing"
)

type fake struct {
	regs map[byte][]byte
	sent [][]Msg
	err  error
}

func newFake() *fake { return &fake{regs: map[byte][]byte{}} }

func (f *fake) Close() error { return nil }

func (f *fake) Transfer(msgs ...Msg) error {
	if f.err != nil {
		return f.err
	}

	recorded := make([]Msg, len(msgs))
	for i, m := range msgs {
		recorded[i] = Msg{Addr: m.Addr, Read: m.Read, Buf: append([]byte(nil), m.Buf...)}
	}
	f.sent = append(f.sent, recorded)

	switch {
	case len(msgs) == 2 && !msgs[0].Read && msgs[1].Read:
		copy(msgs[1].Buf, f.regs[msgs[0].Buf[0]])
	case len(msgs) == 1 && !msgs[0].Read:
		f.regs[msgs[0].Buf[0]] = append([]byte(nil), msgs[0].Buf[1:]...)
	}
	return nil
}

func device(t *testing.T, bus Bus, addr uint16) *Device {
	t.Helper()

	d, err := At(bus, addr)
	if err != nil {
		t.Fatalf("At(%#x): %v", addr, err)
	}
	return d
}

func TestAtRejectsAddressesThatAreNot7Bit(t *testing.T) {
	tests := []struct {
		name string
		addr uint16
		ok   bool
	}{
		{"the light sensor", 0x51, true},
		{"the accelerometer", 0x18, true},
		{"zero is the general call address but still 7-bit", 0x00, true},
		{"the top of the range", 0x77, true},
		{"just past the range", 0x78, false},
		{"a 10-bit address", 0x3ff, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := At(newFake(), tt.addr)
			if (err == nil) != tt.ok {
				t.Errorf("At(%#x) error = %v, want ok = %v", tt.addr, err, tt.ok)
			}
		})
	}
}

func TestReadIsOneTransferWithARepeatedStart(t *testing.T) {
	f := newFake()
	f.regs[0x09] = []byte{0xab, 0xcd}

	d := device(t, f, 0x51)

	var into [2]byte
	if err := d.Read(0x09, into[:]); err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(f.sent) != 1 {
		t.Fatalf("the read took %d transfers, want 1", len(f.sent))
	}
	got := f.sent[0]
	if len(got) != 2 {
		t.Fatalf("the transfer carried %d messages, want 2", len(got))
	}
	if got[0].Read || got[0].Buf[0] != 0x09 {
		t.Errorf("the first leg is %+v, want a write of register 0x09", got[0])
	}
	if !got[1].Read || len(got[1].Buf) != 2 {
		t.Errorf("the second leg is %+v, want a 2 byte read", got[1])
	}
	if into != [2]byte{0xab, 0xcd} {
		t.Errorf("read %#v, want ab cd", into)
	}
}

func TestEveryLegCarriesTheAddress(t *testing.T) {
	f := newFake()
	d := device(t, f, 0x18)

	var into [1]byte
	if err := d.Read(0x00, into[:]); err != nil {
		t.Fatalf("Read: %v", err)
	}

	for i, m := range f.sent[0] {
		if m.Addr != 0x18 {
			t.Errorf("leg %d addressed %#x, want 0x18", i, m.Addr)
		}
	}
}

func TestWritePutsTheRegisterFirst(t *testing.T) {
	f := newFake()
	d := device(t, f, 0x51)

	if err := d.Write(0x03, 0xde, 0xad); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got := f.sent[0]
	if len(got) != 1 || got[0].Read {
		t.Fatalf("the write took %+v, want one write leg", got)
	}
	if want := []byte{0x03, 0xde, 0xad}; string(got[0].Buf) != string(want) {
		t.Errorf("wrote %#v, want %#v", got[0].Buf, want)
	}
}

func TestU16IsLittleEndian(t *testing.T) {
	f := newFake()
	d := device(t, f, 0x51)

	if err := d.WriteU16(0x00, 0x1234); err != nil {
		t.Fatalf("WriteU16: %v", err)
	}
	if got := f.regs[0x00]; len(got) != 2 || got[0] != 0x34 || got[1] != 0x12 {
		t.Fatalf("wrote %#v, want 34 12", got)
	}

	v, err := d.ReadU16(0x00)
	if err != nil {
		t.Fatalf("ReadU16: %v", err)
	}
	if v != 0x1234 {
		t.Errorf("read back %#x, want 0x1234", v)
	}
}

func TestByte(t *testing.T) {
	f := newFake()
	f.regs[0x0f] = []byte{0x42}

	v, err := device(t, f, 0x18).Byte(0x0f)
	if err != nil {
		t.Fatalf("Byte: %v", err)
	}
	if v != 0x42 {
		t.Errorf("read %#x, want 0x42", v)
	}
}

func TestTransferErrorsReachTheCaller(t *testing.T) {
	f := newFake()
	f.err = errors.New("no such device")

	d := device(t, f, 0x51)

	if _, err := d.Byte(0x00); err == nil {
		t.Error("Byte swallowed the bus error")
	}
	if _, err := d.ReadU16(0x00); err == nil {
		t.Error("ReadU16 swallowed the bus error")
	}
	if err := d.Write(0x00, 0x01); err == nil {
		t.Error("Write swallowed the bus error")
	}
}

// The kernel rejects an i2c message carrying no bytes.
func TestEmptyReadDoesNothing(t *testing.T) {
	f := newFake()
	d := device(t, f, 0x51)

	if err := d.Read(0x00, nil); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(f.sent) != 0 {
		t.Errorf("an empty read sent %d transfers, want none", len(f.sent))
	}
}

type present struct{ at map[uint16]bool }

func (p *present) Close() error { return nil }

func (p *present) Transfer(msgs ...Msg) error {
	for _, m := range msgs {
		if !p.at[m.Addr] {
			return errors.New("no acknowledgement")
		}
	}
	return nil
}

func TestProbe(t *testing.T) {
	bus := &present{at: map[uint16]bool{0x51: true}}

	if !Probe(bus, 0x51) {
		t.Error("the chip that is there did not answer")
	}
	if Probe(bus, 0x18) {
		t.Error("an address with nothing on it answered")
	}
}

func TestScanFindsOnlyWhatIsThere(t *testing.T) {
	bus := &present{at: map[uint16]bool{0x0f: true, 0x45: true}}

	got := Scan(bus)
	if len(got) != 2 || got[0] != 0x0f || got[1] != 0x45 {
		t.Errorf("Scan() = %#x, want 0f 45", got)
	}
}

// Addresses below 0x03 and above 0x77 are reserved.
func TestScanStaysInTheAddressableRange(t *testing.T) {
	bus := &present{at: map[uint16]bool{}}
	for addr := uint16(0); addr <= 0xff; addr++ {
		bus.at[addr] = true
	}

	got := Scan(bus)
	if len(got) == 0 {
		t.Fatal("Scan found nothing on a bus answering everywhere")
	}
	if got[0] < 0x03 {
		t.Errorf("Scan probed %#x, below the addressable range", got[0])
	}
	if last := got[len(got)-1]; last > 0x77 {
		t.Errorf("Scan probed %#x, above the addressable range", last)
	}
}

func TestPairs(t *testing.T) {
	rows, err := Pairs([]byte{0x00, 0x01, 0x7f, 0x8c})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0] != [2]byte{0x00, 0x01} || rows[1] != [2]byte{0x7f, 0x8c} {
		t.Errorf("rows %v", rows)
	}
	if _, err := Pairs([]byte{0x00, 0x01, 0x02}); err == nil {
		t.Error("an odd table split without complaint")
	}
}
