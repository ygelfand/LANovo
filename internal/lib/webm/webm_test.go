package webm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"
)

func el(id uint64, body []byte) []byte {
	var out []byte
	switch {
	case id > 0xFFFFFF:
		out = append(out, byte(id>>24), byte(id>>16), byte(id>>8), byte(id))
	case id > 0xFFFF:
		out = append(out, byte(id>>16), byte(id>>8), byte(id))
	case id > 0xFF:
		out = append(out, byte(id>>8), byte(id))
	default:
		out = append(out, byte(id))
	}
	n := uint64(len(body))
	out = append(out, 0x08, byte(n>>48), byte(n>>40), byte(n>>32), byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	out[len(out)-8] = 0x01
	return append(out, body...)
}

func cat(parts ...[]byte) []byte { return bytes.Join(parts, nil) }

func u(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

func f64(v float64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, math.Float64bits(v))
	return b
}

func block(track byte, rel int16, data string) []byte {
	b := []byte{0x80 | track, byte(uint16(rel) >> 8), byte(rel), 0x80}
	return append(b, data...)
}

func stream(segmentSize []byte) []byte {
	head := []byte("OpusHead\x01\x02\x38\x01\x80\xbb\x00\x00\x00\x00\x00")
	tracks := el(idTracks, el(idTrackEntry, cat(
		el(idTrackNumber, u(1)),
		el(idCodecID, []byte("A_OPUS")),
		el(idCodecPrivate, head),
		el(idAudio, cat(el(idSampleRate, f64(48000)), el(idChannels, u(2)))),
	)))
	cluster := el(idCluster, cat(
		el(idTimecode, u(1000)),
		el(idSimpleBlock, block(1, 0, "frame one")),
		el(idSimpleBlock, block(1, 20, "frame two")),
	))
	body := cat(el(idInfo, el(idTimecodeScale, u(1_000_000))), tracks, cluster)

	seg := []byte{0x18, 0x53, 0x80, 0x67}
	if segmentSize == nil {
		seg = el(idSegment, body)
	} else {
		seg = append(append(seg, segmentSize...), body...)
	}
	return cat(el(idEBML, el(0x4282, []byte("webm"))), seg)
}

func TestTheTrackAndItsFramesReadBack(t *testing.T) {
	for name, size := range map[string][]byte{
		"sized segment":   nil,
		"unknown segment": {0x01, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
	} {
		r := NewReader(bytes.NewReader(stream(size)))

		tr, err := r.Track()
		if err != nil {
			t.Fatalf("%s: track: %v", name, err)
		}
		if tr.Number != 1 || tr.Codec != "A_OPUS" || tr.Channels != 2 || tr.Rate != 48000 ||
			!bytes.HasPrefix(tr.Private, []byte("OpusHead")) {
			t.Errorf("%s: track %+v", name, tr)
		}

		want := []Frame{
			{Track: 1, Time: 1000 * 1_000_000, Data: []byte("frame one")},
			{Track: 1, Time: 1020 * 1_000_000, Data: []byte("frame two")},
		}
		for i, w := range want {
			f, err := r.Next()
			if err != nil {
				t.Fatalf("%s: frame %d: %v", name, i, err)
			}
			if f.Track != w.Track || f.Time != w.Time || string(f.Data) != string(w.Data) {
				t.Errorf("%s: frame %d is %+v, want %+v", name, i, f, w)
			}
		}
		if _, err := r.Next(); !errors.Is(err, io.EOF) {
			t.Errorf("%s: after the last frame: %v", name, err)
		}
	}
}

func TestAVideoReaderTakesTheVideoTrack(t *testing.T) {
	tracks := el(idTracks, cat(
		el(idTrackEntry, cat(el(idTrackNumber, u(1)), el(idCodecID, []byte("A_OPUS")))),
		el(idTrackEntry, cat(
			el(idTrackNumber, u(2)),
			el(idCodecID, []byte("V_VP9")),
			el(idVideo, cat(el(idPixelWidth, u(1920)), el(idPixelHeight, u(1080)))),
		)),
	))
	delta := block(2, 33, "delta")
	delta[3] = 0
	cluster := el(idCluster, cat(
		el(idTimecode, u(0)),
		el(idSimpleBlock, block(1, 0, "audio")),
		el(idSimpleBlock, block(2, 0, "key")),
		el(idSimpleBlock, delta),
	))
	s := cat(el(idEBML, el(0x4282, []byte("webm"))), el(idSegment, cat(tracks, cluster)))

	r := NewVideoReader(bytes.NewReader(s))
	tr, err := r.Track()
	if err != nil {
		t.Fatal(err)
	}
	if tr.Number != 2 || tr.Codec != "V_VP9" || tr.Width != 1920 || tr.Height != 1080 {
		t.Errorf("track %+v", tr)
	}
	for _, w := range []Frame{{Track: 2, Data: []byte("key"), Key: true}, {Track: 2, Time: 33_000_000, Data: []byte("delta")}} {
		f, err := r.Next()
		if err != nil {
			t.Fatal(err)
		}
		if f.Track != w.Track || f.Time != w.Time || string(f.Data) != string(w.Data) || f.Key != w.Key {
			t.Errorf("frame %+v, want %+v", f, w)
		}
	}
}

func TestTheIndexJumpsStraightToACluster(t *testing.T) {
	tracks := el(idTracks, el(idTrackEntry, cat(el(idTrackNumber, u(1)), el(idCodecID, []byte("V_VP9")))))
	c1 := el(idCluster, cat(el(idTimecode, u(0)), el(idSimpleBlock, block(1, 0, "first"))))
	c2 := el(idCluster, cat(el(idTimecode, u(5000)), el(idSimpleBlock, block(1, 0, "second"))))
	cues := func(p1, p2 uint64) []byte {
		point := func(tm, pos uint64) []byte {
			return el(idCuePoint, cat(el(idCueTime, u(tm)), el(idCueTrackPositions, cat(el(0xF7, u(1)), el(idCueClusterPosition, u(pos))))))
		}
		return el(idCues, cat(point(0, p1), point(5000, p2)))
	}
	p1 := uint64(len(tracks) + len(cues(0, 0)))
	p2 := p1 + uint64(len(c1))
	head := el(idEBML, el(0x4282, []byte("webm")))
	body := cat(tracks, cues(p1, p2), c1, c2)
	seg := el(idSegment, body)
	file := cat(head, seg)
	segStart := int64(len(head) + len(seg) - len(body))

	r := NewVideoReader(bytes.NewReader(file))
	if _, err := r.Track(); err != nil {
		t.Fatal(err)
	}
	if err := r.Index(); err != nil {
		t.Fatal(err)
	}
	off, at, ok := r.Cue(7000 * 1_000_000)
	if !ok || at != 5000*1_000_000 || off != segStart+int64(p2) {
		t.Fatalf("cue for 7s: offset %d at %d ok %v, want %d at 5s", off, at, ok, segStart+int64(p2))
	}
	r.Restart(bytes.NewReader(file[off:]), off)
	f, err := r.Next()
	if err != nil {
		t.Fatal(err)
	}
	if string(f.Data) != "second" || f.Time != 5000*1_000_000 || !f.Key {
		t.Errorf("after the jump: %+v, want the second cluster's keyframe", f)
	}
	if _, _, ok := r.Cue(-1); ok {
		t.Error("a time before the first cue found one")
	}
}

func TestATruncatedStreamIsAnError(t *testing.T) {
	s := stream(nil)
	r := NewReader(bytes.NewReader(s[:len(s)-4]))
	if _, err := r.Track(); err != nil {
		t.Fatal(err)
	}
	var err error
	for err == nil {
		_, err = r.Next()
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("a cut-off stream ended with %v", err)
	}
}
