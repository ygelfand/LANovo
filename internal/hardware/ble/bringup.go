package ble

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ygelfand/LANovo/internal/board"
)

// Where the chip lives on this device, and where its firmware is kept. The firmware is its own
// read-only partition, mounted per slot, so what is here is the copy for the slot that booted.
const (
	TTY  = "/dev/ttyHS0"
	Node = "/dev/stpbt"

	firmwareDir = "/bt_firmware/image"
	patchFile   = "btfwnpla.tlv"
	nvmFile     = "btnvnpla.bin"
)

// answers is how long the chip is given to say anything. One number everywhere: it answers in
// microseconds or not at all, and a longer wait has never once been the fix.
const answers = 2 * time.Second

// switched is the pause after the rate change, from the kernel's qca_set_baudrate.
const switched = 300 * time.Millisecond

// Listen stops the bring-up after the reset so the caller can read the line raw.
var Listen bool

// Version is what the chip says it is, which is the first thing it says at all.
type Version struct {
	ProductID uint32
	Patch     uint16
	ROM       uint16
	SOC       uint32

	// Raw is the block as it arrived. Kept because the layout is the vendor's and this is how it
	// was worked out: reading it the wrong way round gives numbers that look plausible.
	Raw []byte
}

func (v Version) String() string {
	return fmt.Sprintf("product %#08x patch %#04x rom %#04x soc %#08x  [% x]",
		v.ProductID, v.Patch, v.ROM, v.SOC, v.Raw)
}

// BringUp powers the radio, downloads its firmware and leaves it speaking HCI.
//
// A chip with half a patch answers nothing and looks like a chip that is not there, so every step
// names itself when it fails.
func BringUp() (*Port, Version, error) {
	var none Version

	if board.Current().SoC == board.MediaTek {
		p, err := OpenNode(Node)
		if err != nil {
			return nil, none, err
		}
		if err := Reset(p); err != nil {
			p.Close()
			return nil, none, err
		}
		return p, none, nil
	}

	changed, err := Power(true)
	if err != nil {
		return nil, none, err
	}
	slog.Info("bluetooth radio", "powered", true, "changed", changed)

	p, err := Open(TTY)
	if err != nil {
		return nil, none, err
	}

	v, err := start(p)
	if err != nil {
		p.Close()
		return nil, none, err
	}
	return p, v, nil
}

// start is the exchange, once the chip is powered and the line is open.
func start(p *Port) (Version, error) {
	var none Version

	if err := p.Flush(); err != nil {
		return none, err
	}

	v, err := Ask(p)
	if err != nil {
		return none, fmt.Errorf("reading the version: %w", err)
	}
	slog.Info("bluetooth chip", "version", v)

	if err := Fast(p); err != nil {
		return none, err
	}

	// Nothing acknowledges the rate change, so asking again is what says it took.
	if _, err := Ask(p); err != nil {
		return none, fmt.Errorf("the chip stopped answering after the rate change: %w", err)
	}
	slog.Info("bluetooth line", "rate", "3M")

	for _, f := range []struct {
		what string
		file string
	}{
		{"patch", patchFile},
		{"nvm", nvmFile},
	} {
		raw, err := os.ReadFile(filepath.Join(firmwareDir, f.file))
		if err != nil {
			return none, fmt.Errorf("reading the %s: %w", f.what, err)
		}

		b, err := ReadBlob(raw)
		if err != nil {
			return none, fmt.Errorf("the %s: %w", f.what, err)
		}
		if b.Patch != nil {
			slog.Info("bluetooth patch", "says", b.Patch.String())
		}

		if err := Send(p, b); err != nil {
			return none, fmt.Errorf("sending the %s: %w", f.what, err)
		}
		slog.Info("bluetooth firmware", "sent", f.what, "segments", len(b.Segments()))
	}

	if err := Reset(p); err != nil {
		return none, err
	}

	if Listen {
		return v, nil
	}

	// The loader's version command is gone with the loader. A patched chip is an ordinary
	// controller, so an ordinary answer here is the bring-up proving itself.
	local, err := ReadLocal(p)
	if err != nil {
		return none, fmt.Errorf("the controller did not come back after the reset: %w", err)
	}
	slog.Info("bluetooth controller", "says", local)

	if can, err := ReadFeatures(p); err == nil {
		slog.Info("bluetooth controller", "can", can)
	}

	return v, nil
}

// Local is what the controller says about itself once it is running the firmware.
type Local struct {
	HCIVersion    byte
	HCIRevision   uint16
	LMPVersion    byte
	Manufacturer  uint16
	LMPSubversion uint16
}

func (l Local) String() string {
	return fmt.Sprintf("hci %d rev %d, lmp %d subversion %#04x, manufacturer %d",
		l.HCIVersion, l.HCIRevision, l.LMPVersion, l.LMPSubversion, l.Manufacturer)
}

// ReadLocal asks the controller what it is, the ordinary way.
func ReadLocal(p *Port) (Local, error) {
	cmd, err := command(hciReadLocalVersion)
	if err != nil {
		return Local{}, err
	}
	if _, err := p.Write(cmd); err != nil {
		return Local{}, fmt.Errorf("ble: writing: %w", err)
	}

	said, err := waitReturn(p, hciReadLocalVersion, answers)
	if err != nil {
		return Local{}, err
	}
	if len(said) < 8 {
		return Local{}, fmt.Errorf("ble: a local version of %d bytes", len(said))
	}

	return Local{
		HCIVersion:    said[0],
		HCIRevision:   binary.LittleEndian.Uint16(said[1:]),
		LMPVersion:    said[3],
		Manufacturer:  binary.LittleEndian.Uint16(said[4:]),
		LMPSubversion: binary.LittleEndian.Uint16(said[6:]),
	}, nil
}

// Features is what the controller says it can do, as the eight LMP feature bytes.
//
// Worth asking once because it decides what is worth building on top: whether this is a listening
// device or one that can also be connected to.
type Features [8]byte

// Classic reports BR/EDR support, which is everything that is not Low Energy — a speaker, a
// headset, a phone connecting to play something.
func (f Features) Classic() bool { return f[4]&0x20 == 0 }

// LE reports Low Energy support, which is what a proxy scans with.
func (f Features) LE() bool { return f[4]&0x40 != 0 }

// EDR reports enhanced data rate, the 2 and 3 Mbps basic-rate modes. Audio over a link without it
// is the marginal case rather than the comfortable one.
func (f Features) EDR() bool { return f[3]&0x80 != 0 }

func (f Features) String() string {
	return fmt.Sprintf("classic %v, le %v, edr %v  [% x]", f.Classic(), f.LE(), f.EDR(), f[:])
}

// ReadFeatures asks the controller what it supports.
func ReadFeatures(p *Port) (Features, error) {
	var none Features

	cmd, err := command(hciReadLocalFeatures)
	if err != nil {
		return none, err
	}
	if _, err := p.Write(cmd); err != nil {
		return none, fmt.Errorf("ble: writing: %w", err)
	}

	said, err := waitReturn(p, hciReadLocalFeatures, answers)
	if err != nil {
		return none, err
	}
	if len(said) < 8 {
		return none, fmt.Errorf("ble: %d feature bytes, want 8", len(said))
	}
	return Features(said[:8]), nil
}

// Reset restarts the controller on what was just downloaded. Until this the patch is bytes in the
// chip's memory rather than the firmware it is running.
func Reset(p *Port) error {
	cmd, err := command(hciReset)
	if err != nil {
		return err
	}
	if _, err := p.Write(cmd); err != nil {
		return fmt.Errorf("resetting: %w", err)
	}
	if _, err := waitReturn(p, hciReset, answers); err != nil {
		return fmt.Errorf("resetting: %w", err)
	}

	// Reading rather than sleeping. The firmware announces itself the moment it is running — this
	// is where the sleep protocol starts — and hearing that is what the pause is for.
	p.Settle(switched)
	return nil
}

// waitReturn reads past whatever else is on the line for an opcode's command complete. What it
// skipped goes in the error: a silent line and a line full of other answers are different faults.
func waitReturn(p *Port, opcode uint16, within time.Duration) ([]byte, error) {
	deadline := time.Now().Add(within)

	var saw []string
	for {
		left := time.Until(deadline)
		if left <= 0 {
			if len(saw) == 0 {
				return nil, fmt.Errorf("ble: %#04x was not answered in %v", opcode, within)
			}
			return nil, fmt.Errorf("ble: %#04x was not answered in %v, but the chip said %s",
				opcode, within, strings.Join(saw, ", "))
		}

		e, err := p.Read(left)
		if err != nil {
			return nil, err
		}
		if e.Code == eventCommandComplete && len(e.Params) >= 3 &&
			binary.LittleEndian.Uint16(e.Params[1:]) == opcode {
			return complete(e, opcode)
		}
		saw = append(saw, fmt.Sprintf("%#02x [% x]", e.Code, e.Params))
	}
}

// Ask reads the chip's version, which is also how to find out whether it is listening at all.
func Ask(p *Port) (Version, error) {
	cmd, err := version()
	if err != nil {
		return Version{}, err
	}

	if _, err := p.Write(cmd); err != nil {
		return Version{}, fmt.Errorf("ble: writing: %w", err)
	}

	said, err := waitFor(p, edlVersionResult, answers)
	if err != nil {
		return Version{}, err
	}
	if len(said) < 12 {
		return Version{}, fmt.Errorf("ble: a version of %d bytes", len(said))
	}

	// Four byte product, all little endian: read this way the rom matches the patch file, which
	// is what says the layout and the firmware are both right for this chip.
	return Version{
		ProductID: binary.LittleEndian.Uint32(said[0:]),
		Patch:     binary.LittleEndian.Uint16(said[4:]),
		ROM:       binary.LittleEndian.Uint16(said[6:]),
		SOC:       binary.LittleEndian.Uint32(said[8:]),
		Raw:       said,
	}, nil
}

// waitFor reads past whatever else is on the line for a vendor event. The chip leads with a
// command complete and puts the answer behind it, and volunteers events of its own.
func waitFor(p *Port, want byte, within time.Duration) ([]byte, error) {
	deadline := time.Now().Add(within)

	for {
		left := time.Until(deadline)
		if left <= 0 {
			return nil, fmt.Errorf("ble: no answer to %#02x in %v", want, within)
		}

		e, err := p.Read(left)
		if err != nil {
			return nil, err
		}
		if e.Code != eventVendor || len(e.Params) < 2 || e.Params[1] != want {
			continue
		}
		return answered(e, want)
	}
}

// Fast raises the line to 3 Mbps, where the download happens.
//
// Not an optimisation: at the reset rate the chip's buffer fills and it stops taking bytes at
// segment 18 of 128. Nothing answers this command, so the next thing said at the new rate is the
// only proof. Flow control off across the change, and the command fully out before the local end
// moves — a write returns when the kernel has it, not the wire.
func Fast(p *Port) error {
	cmd, err := command(baudOpcode, baud3M)
	if err != nil {
		return err
	}

	if err := p.Flow(false); err != nil {
		return err
	}
	if _, err := p.Write(cmd); err != nil {
		return fmt.Errorf("asking for the higher rate: %w", err)
	}
	if err := p.Drain(answers); err != nil {
		return err
	}
	time.Sleep(switched)

	if err := p.Speed(fast); err != nil {
		return err
	}
	return p.Flow(true)
}

// Send pushes a blob one segment at a time. Whether each is answered is the blob's to say; a
// silent download is paced by flow control.
func Send(p *Port, b Blob) error {
	segments := b.Segments()
	acked := b.Acked()

	for i, piece := range segments {
		cmd, err := download(piece)
		if err != nil {
			return err
		}

		if _, err := p.Write(cmd); err != nil {
			return fmt.Errorf("segment %d of %d: %w", i+1, len(segments), err)
		}

		// A silent blob is never answered, its last segment included: download 0x03 tells the chip
		// to skip both the vendor event and the completion for the whole download. Waiting on the
		// end of it is a two second stall and then a failure with the patch already delivered.
		if !acked {
			continue
		}
		if err := taken(p); err != nil {
			return fmt.Errorf("segment %d of %d: %w", i+1, len(segments), err)
		}
	}

	// Nothing was read for a silent blob, so anything the chip has to say about it is still coming
	// while the next one is already going out, where it is read as that one's first answer and
	// blamed on it. Taken here it is only logged, and the next blob starts on an empty line.
	if !acked {
		quiet(p)
	}
	return nil
}

// trailing is how long a silent download is given to say anything before the next one starts.
var trailing = 300 * time.Millisecond

// quiet reads whatever is left on the line, so it cannot be mistaken for the next answer.
func quiet(p *Port) {
	for {
		e, err := p.Read(trailing)
		if err != nil {
			return
		}
		slog.Info("bluetooth after a silent download",
			"event", fmt.Sprintf("%#02x", e.Code), "params", fmt.Sprintf("% x", e.Params))
	}
}
