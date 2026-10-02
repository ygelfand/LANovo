package cenc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const (
	SchemeNone = ""
	SchemeCENC = "cenc"
	SchemeCBCS = "cbcs"
)

type Subsample struct{ Clear, Encrypted uint32 }

type Track struct {
	Format        string
	Width, Height int
	Rate          int
	Channels      int
	Timescale     uint32
	Config        []byte
	SPS, PPS      [][]byte
	LengthSize    int
	Scheme        string
	KeyID         [16]byte
	IVSize        int
	ConstantIV    []byte
	Crypt, Skip   uint8
	PSSH          [][]byte
}

type Sample struct {
	Data       []byte
	At         time.Duration
	Duration   time.Duration
	Key        bool
	KeyID      [16]byte
	IV         [16]byte
	Subsamples []Subsample
	Encrypted  bool
}

var errShort = errors.New("cenc: truncated box")

type box struct {
	kind  string
	body  []byte
	start int
}

func boxes(b []byte, base int) ([]box, error) {
	var out []box
	for off := 0; off+8 <= len(b); {
		size := int(binary.BigEndian.Uint32(b[off:]))
		kind := string(b[off+4 : off+8])
		head := 8
		switch size {
		case 0:
			size = len(b) - off
		case 1:
			if off+16 > len(b) {
				return out, errShort
			}
			size = int(binary.BigEndian.Uint64(b[off+8:]))
			head = 16
		}
		if size < head || size > len(b)-off {
			return out, errShort
		}
		out = append(out, box{kind: kind, body: b[off+head : off+size], start: base + off})
		off += size
	}
	return out, nil
}

func child(bs []box, kind string) (box, bool) {
	for _, b := range bs {
		if b.kind == kind {
			return b, true
		}
	}
	return box{}, false
}

func children(b box, skip int) []box {
	if len(b.body) < skip {
		return nil
	}
	out, _ := boxes(b.body[skip:], 0)
	return out
}

func path(bs []box, kinds ...string) (box, bool) {
	var cur box
	for i, k := range kinds {
		b, ok := child(bs, k)
		if !ok {
			return box{}, false
		}
		cur = b
		if i < len(kinds)-1 {
			skip := 0
			if k == "stsd" {
				skip = 8
			}
			bs = children(b, skip)
		}
	}
	return cur, true
}

func ParseInit(b []byte) (Track, error) {
	top, err := boxes(b, 0)
	if err != nil {
		return Track{}, err
	}
	moov, ok := child(top, "moov")
	if !ok {
		return Track{}, errors.New("cenc: no moov")
	}
	var t Track
	mv := children(moov, 0)
	for _, p := range mv {
		if p.kind == "pssh" {
			t.PSSH = append(t.PSSH, wrap("pssh", p.body))
		}
	}
	trak, ok := child(mv, "trak")
	if !ok {
		return Track{}, errors.New("cenc: no trak")
	}
	tr := children(trak, 0)
	if mdhd, ok := path(tr, "mdia", "mdhd"); ok && len(mdhd.body) >= 24 {
		at := 12
		if mdhd.body[0] == 1 {
			at = 20
		}
		t.Timescale = binary.BigEndian.Uint32(mdhd.body[at:])
	}
	stsd, ok := path(tr, "mdia", "minf", "stbl", "stsd")
	if !ok {
		return Track{}, errors.New("cenc: no stsd")
	}
	entries := children(stsd, 8)
	if len(entries) == 0 {
		return Track{}, errors.New("cenc: empty stsd")
	}
	e := entries[0]
	var inner []box
	switch e.kind {
	case "encv", "avc1", "avc3", "hev1", "hvc1", "vp09":
		if len(e.body) < 78 {
			return Track{}, errShort
		}
		t.Width = int(binary.BigEndian.Uint16(e.body[24:]))
		t.Height = int(binary.BigEndian.Uint16(e.body[26:]))
		inner = children(e, 78)
	case "enca", "mp4a":
		if len(e.body) < 28 {
			return Track{}, errShort
		}
		t.Channels = int(binary.BigEndian.Uint16(e.body[16:]))
		t.Rate = int(binary.BigEndian.Uint32(e.body[24:]) >> 16)
		inner = children(e, 28)
	default:
		return Track{}, fmt.Errorf("cenc: sample entry %q", e.kind)
	}
	t.Format = e.kind
	if sinf, ok := child(inner, "sinf"); ok {
		si := children(sinf, 0)
		if frma, ok := child(si, "frma"); ok && len(frma.body) >= 4 {
			t.Format = string(frma.body[:4])
		}
		if schm, ok := child(si, "schm"); ok && len(schm.body) >= 8 {
			t.Scheme = string(schm.body[4:8])
		}
		if tenc, ok := path(si, "schi", "tenc"); ok {
			if err := t.readTenc(tenc.body); err != nil {
				return Track{}, err
			}
		}
	}
	if avcc, ok := child(inner, "avcC"); ok {
		t.Config = avcc.body
		t.readAVCC(avcc.body)
	}
	if esds, ok := child(inner, "esds"); ok {
		t.Config = audioConfig(esds.body)
	}
	return t, nil
}

func wrap(kind string, body []byte) []byte {
	out := make([]byte, 8+len(body))
	binary.BigEndian.PutUint32(out, uint32(len(out)))
	copy(out[4:], kind)
	copy(out[8:], body)
	return out
}

func (t *Track) readTenc(b []byte) error {
	if len(b) < 24 {
		return errShort
	}
	if b[0] > 0 {
		t.Crypt, t.Skip = b[5]>>4, b[5]&0x0f
	}
	protected := b[6] != 0
	t.IVSize = int(b[7])
	copy(t.KeyID[:], b[8:24])
	if protected && t.IVSize == 0 && len(b) >= 25 {
		n := int(b[24])
		if len(b) >= 25+n {
			t.ConstantIV = append([]byte(nil), b[25:25+n]...)
		}
	}
	if !protected {
		t.Scheme = SchemeNone
	}
	return nil
}

func (t *Track) readAVCC(b []byte) {
	if len(b) < 6 {
		return
	}
	t.LengthSize = int(b[4]&3) + 1
	n := int(b[5] & 0x1f)
	off := 6
	for range n {
		if off+2 > len(b) {
			return
		}
		l := int(binary.BigEndian.Uint16(b[off:]))
		if off+2+l > len(b) {
			return
		}
		t.SPS = append(t.SPS, b[off+2:off+2+l])
		off += 2 + l
	}
	if off >= len(b) {
		return
	}
	n = int(b[off])
	off++
	for range n {
		if off+2 > len(b) {
			return
		}
		l := int(binary.BigEndian.Uint16(b[off:]))
		if off+2+l > len(b) {
			return
		}
		t.PPS = append(t.PPS, b[off+2:off+2+l])
		off += 2 + l
	}
}

func audioConfig(b []byte) []byte {
	if len(b) < 4 {
		return nil
	}
	b = b[4:]
	for len(b) > 2 {
		tag := b[0]
		n, used := descLen(b[1:])
		if used == 0 {
			return nil
		}
		body := b[1+used:]
		if n > len(body) {
			n = len(body)
		}
		switch tag {
		case 3:
			if len(body) < 3 {
				return nil
			}
			flags := body[2]
			skip := 3
			if flags&0x80 != 0 {
				skip += 2
			}
			if flags&0x40 != 0 && len(body) > skip {
				skip += 1 + int(body[skip])
			}
			if flags&0x20 != 0 {
				skip += 2
			}
			if skip > n {
				return nil
			}
			b = body[skip:n]
		case 4:
			if n < 13 {
				return nil
			}
			b = body[13:n]
		case 5:
			return append([]byte(nil), body[:n]...)
		default:
			b = body[n:]
		}
	}
	return nil
}

func descLen(b []byte) (int, int) {
	n := 0
	for i := 0; i < 4 && i < len(b); i++ {
		n = n<<7 | int(b[i]&0x7f)
		if b[i]&0x80 == 0 {
			return n, i + 1
		}
	}
	return 0, 0
}
