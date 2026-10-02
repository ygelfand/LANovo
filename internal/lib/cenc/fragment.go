package cenc

import (
	"encoding/binary"
	"errors"
	"time"
)

const (
	tfhdBase      = 0x000001
	tfhdDuration  = 0x000008
	tfhdSize      = 0x000010
	tfhdFlags     = 0x000020
	tfhdBaseMoof  = 0x020000
	trunOffset    = 0x000001
	trunFirst     = 0x000004
	trunDuration  = 0x000100
	trunSize      = 0x000200
	trunFlags     = 0x000400
	trunCTS       = 0x000800
	sencSubsample = 0x000002
	nonSync       = 0x00010000
)

type fragment struct {
	limit       int
	base        int
	defDuration uint32
	defSize     uint32
	defFlags    uint32
	decode      uint64
	samples     []raw
	auxDefault  int
	auxCount    uint32
	auxTable    []byte
	auxOffset   int
	haveAux     bool
	senc        []byte
	sencSubs    bool
	groups      []seig
	groupOf     []int
	pssh        [][]byte
}

type raw struct {
	offset   int
	size     int
	duration uint32
	flags    uint32
	cts      int32
}

type seig struct {
	protected bool
	ivSize    int
	keyID     [16]byte
	constIV   []byte
}

func ParseFragment(t Track, b []byte) ([]Sample, [][]byte, error) {
	top, err := boxes(b, 0)
	if err != nil {
		return nil, nil, err
	}
	var out []Sample
	var pssh [][]byte
	for _, moof := range top {
		if moof.kind != "moof" {
			continue
		}
		f, err := readMoof(moof, len(b))
		if err != nil {
			return nil, nil, err
		}
		pssh = append(pssh, f.pssh...)
		samples, err := f.resolve(t, b)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, samples...)
	}
	return out, pssh, nil
}

func Parse(b []byte) (Track, []Sample, error) {
	t, err := ParseInit(b)
	if err != nil {
		return Track{}, nil, err
	}
	samples, _, err := ParseFragment(t, b)
	return t, samples, err
}

func readMoof(moof box, limit int) (*fragment, error) {
	f := &fragment{limit: limit}
	mb := children(moof, 0)
	moofStart := moof.start
	for _, p := range mb {
		if p.kind == "pssh" {
			f.pssh = append(f.pssh, wrap("pssh", p.body))
		}
	}
	traf, ok := child(mb, "traf")
	if !ok {
		return nil, errors.New("cenc: no traf")
	}
	var sgpd, sbgp []byte
	for _, x := range children(traf, 0) {
		body := x.body
		if len(body) < 4 {
			continue
		}
		version, flags := body[0], binary.BigEndian.Uint32(body)&0xffffff
		switch x.kind {
		case "tfhd":
			f.base = moofStart
			off := 8
			if flags&tfhdBase != 0 && len(body) >= off+8 {
				f.base = int(binary.BigEndian.Uint64(body[off:]))
				off += 8
			}
			if flags&0x000002 != 0 {
				off += 4
			}
			if flags&tfhdDuration != 0 && len(body) >= off+4 {
				f.defDuration = binary.BigEndian.Uint32(body[off:])
				off += 4
			}
			if flags&tfhdSize != 0 && len(body) >= off+4 {
				f.defSize = binary.BigEndian.Uint32(body[off:])
				off += 4
			}
			if flags&tfhdFlags != 0 && len(body) >= off+4 {
				f.defFlags = binary.BigEndian.Uint32(body[off:])
			}
		case "tfdt":
			if version == 1 && len(body) >= 12 {
				f.decode = binary.BigEndian.Uint64(body[4:])
			} else if len(body) >= 8 {
				f.decode = uint64(binary.BigEndian.Uint32(body[4:]))
			}
		case "trun":
			if err := f.readTrun(version, flags, body); err != nil {
				return nil, err
			}
		case "saiz":
			off := 4
			if flags&1 != 0 {
				off += 8
			}
			if len(body) < off+5 {
				continue
			}
			f.auxDefault = int(body[off])
			f.auxCount = binary.BigEndian.Uint32(body[off+1:])
			f.auxTable = body[off+5:]
		case "saio":
			off := 4
			if flags&1 != 0 {
				off += 8
			}
			if len(body) < off+8 {
				continue
			}
			n := binary.BigEndian.Uint32(body[off:])
			off += 4
			if n == 0 {
				continue
			}
			if version == 1 && len(body) >= off+8 {
				f.auxOffset = int(binary.BigEndian.Uint64(body[off:]))
			} else {
				f.auxOffset = int(binary.BigEndian.Uint32(body[off:]))
			}
			f.haveAux = true
		case "senc":
			f.senc = body[4:]
			f.sencSubs = flags&sencSubsample != 0
		case "sgpd":
			sgpd = body
		case "sbgp":
			sbgp = body
		}
	}
	f.readGroups(sgpd, sbgp)
	return f, nil
}

func (f *fragment) readTrun(version uint8, flags uint32, b []byte) error {
	if len(b) < 8 {
		return errShort
	}
	n := binary.BigEndian.Uint32(b[4:])
	off := 8
	data := 0
	if flags&trunOffset != 0 {
		if len(b) < off+4 {
			return errShort
		}
		data = int(int32(binary.BigEndian.Uint32(b[off:])))
		off += 4
	}
	first, haveFirst := uint32(0), false
	if flags&trunFirst != 0 {
		if len(b) < off+4 {
			return errShort
		}
		first, haveFirst = binary.BigEndian.Uint32(b[off:]), true
		off += 4
	}
	fields := 4 * (bits(flags&trunDuration) + bits(flags&trunSize) + bits(flags&trunFlags) + bits(flags&trunCTS))
	if fields > 0 && uint64(n)*uint64(fields) > uint64(len(b)-off) {
		return errShort
	}
	if fields == 0 && uint64(n)*uint64(max(f.defSize, 1)) > uint64(f.limit) {
		return errShort
	}
	pos := f.base + data
	if len(f.samples) > 0 && flags&trunOffset == 0 {
		last := f.samples[len(f.samples)-1]
		pos = last.offset + last.size
	}
	for i := range n {
		s := raw{duration: f.defDuration, size: int(f.defSize), flags: f.defFlags}
		if flags&trunDuration != 0 {
			if len(b) < off+4 {
				return errShort
			}
			s.duration = binary.BigEndian.Uint32(b[off:])
			off += 4
		}
		if flags&trunSize != 0 {
			if len(b) < off+4 {
				return errShort
			}
			s.size = int(binary.BigEndian.Uint32(b[off:]))
			off += 4
		}
		if flags&trunFlags != 0 {
			if len(b) < off+4 {
				return errShort
			}
			s.flags = binary.BigEndian.Uint32(b[off:])
			off += 4
		} else if i == 0 && haveFirst {
			s.flags = first
		}
		if flags&trunCTS != 0 {
			if len(b) < off+4 {
				return errShort
			}
			v := binary.BigEndian.Uint32(b[off:])
			if version == 0 {
				s.cts = int32(min(v, 1<<31-1))
			} else {
				s.cts = int32(v)
			}
			off += 4
		}
		s.offset = pos
		pos += s.size
		f.samples = append(f.samples, s)
	}
	return nil
}

func (f *fragment) readGroups(sgpd, sbgp []byte) {
	if len(sgpd) < 12 || string(sgpd[4:8]) != "seig" {
		return
	}
	version := sgpd[0]
	off := 8
	defLen := 0
	if version == 1 {
		defLen = int(binary.BigEndian.Uint32(sgpd[off:]))
		off += 4
	} else if version >= 2 {
		off += 4
	}
	if len(sgpd) < off+4 {
		return
	}
	n := int(binary.BigEndian.Uint32(sgpd[off:]))
	off += 4
	for range n {
		l := defLen
		if version == 1 && defLen == 0 {
			if len(sgpd) < off+4 {
				return
			}
			l = int(binary.BigEndian.Uint32(sgpd[off:]))
			off += 4
		}
		if l == 0 {
			l = 20
		}
		if l < 20 || l > len(sgpd)-off {
			return
		}
		e := sgpd[off : off+l]
		g := seig{protected: e[2] != 0, ivSize: int(e[3])}
		copy(g.keyID[:], e[4:20])
		if g.protected && g.ivSize == 0 && len(e) > 20 {
			c := int(e[20])
			if len(e) >= 21+c {
				g.constIV = append([]byte(nil), e[21:21+c]...)
			}
		}
		f.groups = append(f.groups, g)
		off += l
	}
	if len(sbgp) < 12 || string(sbgp[4:8]) != "seig" {
		return
	}
	off = 8
	if sbgp[0] == 1 {
		off += 4
	}
	if len(sbgp) < off+4 {
		return
	}
	entries := int(binary.BigEndian.Uint32(sbgp[off:]))
	off += 4
	for range entries {
		if len(sbgp) < off+8 {
			return
		}
		count := int(binary.BigEndian.Uint32(sbgp[off:]))
		index := int(binary.BigEndian.Uint32(sbgp[off+4:]))
		off += 8
		if index > 0x10000 {
			index -= 0x10000
		}
		for range count {
			if len(f.groupOf) >= len(f.samples) {
				return
			}
			f.groupOf = append(f.groupOf, index)
		}
	}
}

func (f *fragment) resolve(t Track, file []byte) ([]Sample, error) {
	scale := t.Timescale
	if scale == 0 {
		scale = 1
	}
	at := func(ticks int64) time.Duration {
		s := int64(scale)
		return time.Duration(ticks/s)*time.Second + time.Duration(ticks%s)*time.Second/time.Duration(s)
	}
	aux := f.auxRecords(t, file)
	out := make([]Sample, 0, len(f.samples))
	dts := int64(f.decode)
	for i, r := range f.samples {
		if r.offset < 0 || r.size < 0 || r.offset > len(file) || r.size > len(file)-r.offset {
			return out, errShort
		}
		s := Sample{
			Data:     file[r.offset : r.offset+r.size],
			At:       at(dts + int64(r.cts)),
			Duration: at(int64(r.duration)),
			Key:      r.flags&nonSync == 0,
		}
		dts += int64(r.duration)
		protected, ivSize, kid, constIV := t.Scheme != SchemeNone, t.IVSize, t.KeyID, t.ConstantIV
		if i < len(f.groupOf) && f.groupOf[i] > 0 && f.groupOf[i] <= len(f.groups) {
			g := f.groups[f.groupOf[i]-1]
			protected, ivSize, kid, constIV = g.protected, g.ivSize, g.keyID, g.constIV
		}
		if protected {
			s.Encrypted = true
			s.KeyID = kid
			if ivSize == 0 {
				copy(s.IV[:], constIV)
			} else if i < len(aux) {
				a := aux[i]
				if len(a) >= ivSize {
					copy(s.IV[:], a[:ivSize])
					s.Subsamples = subsamples(a[ivSize:])
				}
			}
		}
		out = append(out, s)
	}
	return out, nil
}

func (f *fragment) auxRecords(t Track, file []byte) [][]byte {
	if f.senc != nil {
		return sencRecords(f.senc, f.sencSubs, t.IVSize, len(f.samples))
	}
	if !f.haveAux {
		return nil
	}
	pos := f.base + f.auxOffset
	var out [][]byte
	for i := uint64(0); i < uint64(f.auxCount) && len(out) < len(f.samples); i++ {
		n := f.auxDefault
		if n == 0 {
			if i >= uint64(len(f.auxTable)) {
				return out
			}
			n = int(f.auxTable[i])
		}
		if pos < 0 || pos > len(file) || n > len(file)-pos {
			return out
		}
		out = append(out, file[pos:pos+n])
		pos += n
	}
	return out
}

func sencRecords(b []byte, subs bool, ivSize, most int) [][]byte {
	if len(b) < 4 {
		return nil
	}
	n := min(uint64(binary.BigEndian.Uint32(b)), uint64(most))
	off := 4
	out := make([][]byte, 0, n)
	for range n {
		start := off
		off += ivSize
		if subs {
			if off+2 > len(b) {
				return out
			}
			off += 2 + 6*int(binary.BigEndian.Uint16(b[off:]))
		}
		if off > len(b) {
			return out
		}
		out = append(out, b[start:off])
	}
	return out
}

func subsamples(b []byte) []Subsample {
	if len(b) < 2 {
		return nil
	}
	n := int(binary.BigEndian.Uint16(b))
	b = b[2:]
	out := make([]Subsample, 0, n)
	for i := 0; i < n && len(b) >= 6; i++ {
		out = append(out, Subsample{Clear: uint32(binary.BigEndian.Uint16(b)), Encrypted: binary.BigEndian.Uint32(b[2:])})
		b = b[6:]
	}
	return out
}

func bits(v uint32) int {
	if v != 0 {
		return 1
	}
	return 0
}
