package ble

import (
	"encoding/binary"
	"fmt"
)

// The vendor command the whole bring-up goes through, and the requests it carries.
const (
	edlOpcode = 0xfc00

	edlVersionReq = 0x19
	edlTLVReq     = 0x1e
)

// Changing the line rate is its own vendor command rather than a request under the one above.
const (
	baudOpcode = 0xfc48
	baud3M     = 0x0e
)

// What the chip answers with. A vendor event carries the request it is answering, so a reply is
// matched by what it says rather than by its arrival order.
const (
	edlRequestResult = 0x00

	// The reply to a version request is not the request's own code: 0x19 asks, 0x02 answers.
	edlVersionResult = 0x02

	edlDownloadDone = 0x04
)

// What a TLV blob holds. The patch is code for the chip; the NVM is the board's own settings, and
// the two are separate files downloaded the same way.
const (
	tlvPatch = 1
	tlvNVM   = 2
)

// segment is the most of a blob one command carries. The command also holds the request byte and a
// length, and an HCI command's parameters are a byte long, so this is what is left.
const segment = 243

// Blob is a patch or an NVM file, read but not yet sent. The chip parses it itself, so the bytes
// go over the wire as they are on disk; the header is read here only to refuse a wrong file.
type Blob struct {
	Type byte
	Raw  []byte

	// Patch is the header the chip's own loader reads, present only on a patch.
	Patch *Patch
}

// Patch is what a patch file says about itself.
type Patch struct {
	TotalSize  uint32
	DataLength uint32
	Format     byte
	Signature  byte

	// Download is how the chip will behave while it is being sent this patch.
	Download byte

	ProductID uint16
	ROMBuild  uint16
	Version   uint16
	Entry     uint32
}

// What a patch's download byte says the chip will answer with. Only the two ends are used: either
// every segment is acknowledged or none of them are.
const (
	sayEverything = 0x00
	saySilent     = 0x03
)

func (p Patch) String() string {
	return fmt.Sprintf("product %#04x rom %d patch %d entry %#08x, %d bytes, download %#02x",
		p.ProductID, p.ROMBuild, p.Version, p.Entry, p.DataLength, p.Download)
}

// ReadBlob reads a TLV file and says what it is.
//
// The four byte header is a type in the low byte and a length in the three above it, so a file is
// self-describing and a truncated one is caught here rather than by the chip refusing a segment
// halfway through.
func ReadBlob(raw []byte) (Blob, error) {
	const header = 4

	if len(raw) < header {
		return Blob{}, fmt.Errorf("ble: %d bytes is too short to be a TLV", len(raw))
	}

	at := binary.LittleEndian.Uint32(raw)
	b := Blob{Type: byte(at & 0xff), Raw: raw}

	if want := int(at>>8) + header; len(raw) < want {
		return Blob{}, fmt.Errorf("ble: the TLV says %d bytes and the file is %d", want, len(raw))
	}

	switch b.Type {
	case tlvNVM:
		return b, nil
	case tlvPatch:
	default:
		return Blob{}, fmt.Errorf("ble: TLV type %d is neither a patch nor an NVM", b.Type)
	}

	// The patch header sits straight after the TLV header, and the chip's loader reads it.
	const patchHeader = 28
	if len(raw) < header+patchHeader {
		return Blob{}, fmt.Errorf("ble: a patch with no header in %d bytes", len(raw))
	}

	in := raw[header:]
	b.Patch = &Patch{
		TotalSize:  binary.LittleEndian.Uint32(in[0:]),
		DataLength: binary.LittleEndian.Uint32(in[4:]),
		Format:     in[8],
		Signature:  in[9],
		Download:   in[10],
		ProductID:  binary.LittleEndian.Uint16(in[12:]),
		ROMBuild:   binary.LittleEndian.Uint16(in[14:]),
		Version:    binary.LittleEndian.Uint16(in[16:]),
		Entry:      binary.LittleEndian.Uint32(in[20:]),
	}
	return b, nil
}

// Acked says whether the chip answers each segment. The patch header's download byte decides it —
// 0x03 is silent for the whole download. An NVM has no header and is always answered.
func (b Blob) Acked() bool {
	return b.Patch == nil || b.Patch.Download == sayEverything
}

// Segments is the blob as the commands that carry it.
//
// The whole file goes, header included, in pieces of at most segment bytes. The chip reassembles
// them and reads the header itself, so nothing here decides where a piece may be cut.
func (b Blob) Segments() [][]byte {
	out := make([][]byte, 0, len(b.Raw)/segment+1)

	for at := 0; at < len(b.Raw); at += segment {
		out = append(out, b.Raw[at:min(at+segment, len(b.Raw))])
	}
	return out
}

// download is the command that sends one segment.
func download(piece []byte) ([]byte, error) {
	if len(piece) > segment {
		return nil, fmt.Errorf("ble: a %d byte segment, more than the %d one command holds",
			len(piece), segment)
	}
	return command(edlOpcode, append([]byte{edlTLVReq, byte(len(piece))}, piece...)...)
}

// version asks the chip what it is, which is the first thing said to it and the thing that says it
// is listening at all.
func version() ([]byte, error) { return command(edlOpcode, edlVersionReq) }

// Why the chip refuses a segment. The vendor's downloader logs these and carries on regardless.
var refusals = map[byte]string{
	1: "the patch length is wrong",
	2: "the patch version is wrong",
	3: "the patch failed its CRC",
	4: "the patch data is not valid",
	5: "the TLV type is wrong",
}

func refusal(status byte) string {
	if why, ok := refusals[status]; ok {
		return why
	}
	return fmt.Sprintf("status %#02x", status)
}

// taken reads one segment's acknowledgement, which is two events: a vendor event carrying the
// status, and a command complete whose opcode is zero. Reading one of the two leaves the caller a
// packet behind per segment. Either order, since nothing says which comes first.
func taken(p *Port) error {
	var said, done bool

	for range 2 {
		e, err := p.Read(answers)
		if err != nil {
			return err
		}

		switch e.Code {
		case eventVendor:
			status, err := answered(e, edlDownloadDone)
			if err != nil {
				return err
			}
			if len(status) > 0 && status[0] != 0 {
				return fmt.Errorf("ble: the chip refused the segment: %s", refusal(status[0]))
			}
			said = true
		case eventCommandComplete:
			if len(e.Params) >= 4 && e.Params[3] != 0 {
				return fmt.Errorf("ble: the segment was refused, status %#02x", e.Params[3])
			}
			done = true
		}
	}

	if !said || !done {
		return fmt.Errorf("ble: the segment was answered twice but not with both halves")
	}
	return nil
}

// answered reads a vendor event and checks it is the chip agreeing to what was asked. The event
// names the request, so a reply to something else is out of order rather than a failure.
func answered(e event, want byte) ([]byte, error) {
	if e.Code != eventVendor {
		return nil, fmt.Errorf("ble: event %#02x, want a vendor event", e.Code)
	}
	if len(e.Params) < 2 {
		return nil, errShort
	}

	if got := e.Params[0]; got != edlRequestResult {
		return nil, fmt.Errorf("ble: the chip reports %#02x rather than a result", got)
	}
	if got := e.Params[1]; got != want {
		return nil, fmt.Errorf("ble: an answer to %#02x while waiting on %#02x", got, want)
	}
	return e.Params[2:], nil
}
