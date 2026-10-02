package cenc

import (
	"encoding/binary"
	"fmt"
)

var startCode = []byte{0, 0, 0, 1}

func (t Track) AnnexB(s Sample) ([]byte, []Subsample, error) {
	if t.LengthSize != 4 {
		return nil, nil, fmt.Errorf("cenc: %d-byte NAL lengths", t.LengthSize)
	}
	var head []byte
	if s.Key {
		for _, p := range append(append([][]byte{}, t.SPS...), t.PPS...) {
			head = append(append(head, startCode...), p...)
		}
	}
	out := make([]byte, 0, len(head)+len(s.Data))
	out = append(out, head...)
	body := len(out)
	out = append(out, s.Data...)
	for off := body; off+4 <= len(out); {
		n := binary.BigEndian.Uint32(out[off:])
		if uint64(n) > uint64(len(out)-off-4) {
			return nil, nil, fmt.Errorf("cenc: NAL overruns the sample")
		}
		copy(out[off:], startCode)
		off += 4 + int(n)
	}
	subs := s.Subsamples
	if s.Encrypted && len(head) > 0 {
		subs = append([]Subsample(nil), subs...)
		if len(subs) == 0 {
			subs = []Subsample{{Clear: uint32(len(head)), Encrypted: uint32(len(s.Data))}}
		} else {
			subs[0].Clear += uint32(len(head))
		}
	}
	return out, subs, nil
}
