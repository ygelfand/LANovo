package cenc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

type Fragment struct {
	Offset, Size int64
	At, Duration time.Duration
	Index        bool
}

func ParseIndex(b []byte, offset int64) ([]Fragment, error) {
	return parseIndex(b, func(end int) int64 { return offset + int64(end) })
}

func ParseIndexAt(b []byte, anchor int64) ([]Fragment, error) {
	return parseIndex(b, func(int) int64 { return anchor })
}

func parseIndex(b []byte, anchor func(end int) int64) ([]Fragment, error) {
	top, err := boxes(b, 0)
	if err != nil {
		return nil, err
	}
	sidx, ok := child(top, "sidx")
	if !ok {
		return nil, errors.New("cenc: no sidx")
	}
	body := sidx.body
	end := sidx.start + 8 + len(body)
	if len(body) < 12 {
		return nil, errors.New("cenc: short sidx")
	}
	version := body[0]
	scale := binary.BigEndian.Uint32(body[8:12])
	if scale == 0 {
		return nil, errors.New("cenc: sidx timescale 0")
	}
	p := 12
	var earliest, first uint64
	if version == 0 {
		if len(body) < p+8 {
			return nil, errors.New("cenc: short sidx")
		}
		earliest = uint64(binary.BigEndian.Uint32(body[p:]))
		first = uint64(binary.BigEndian.Uint32(body[p+4:]))
		p += 8
	} else {
		if len(body) < p+16 {
			return nil, errors.New("cenc: short sidx")
		}
		earliest = binary.BigEndian.Uint64(body[p:])
		first = binary.BigEndian.Uint64(body[p+8:])
		p += 16
	}
	if len(body) < p+4 {
		return nil, errors.New("cenc: short sidx")
	}
	count := int(binary.BigEndian.Uint16(body[p+2:]))
	p += 4
	if len(body) < p+12*count {
		return nil, errors.New("cenc: short sidx")
	}
	at := anchor(end) + int64(first)
	t := earliest
	out := make([]Fragment, 0, count)
	for i := 0; i < count; i++ {
		ref := binary.BigEndian.Uint32(body[p:])
		dur := binary.BigEndian.Uint32(body[p+4:])
		p += 12
		size := int64(ref & 0x7fffffff)
		out = append(out, Fragment{
			Offset:   at,
			Size:     size,
			At:       time.Duration(t) * time.Second / time.Duration(scale),
			Duration: time.Duration(dur) * time.Second / time.Duration(scale),
			Index:    ref>>31 == 1,
		})
		at += size
		t += uint64(dur)
	}
	return out, nil
}

func Boxes(b []byte) string {
	top, _ := boxes(b, 0)
	out := ""
	for i, x := range top {
		if i == 8 {
			out += " …"
			break
		}
		if out != "" {
			out += " "
		}
		out += fmt.Sprintf("%s@%d+%d", x.kind, x.start, len(x.body))
	}
	return out
}
