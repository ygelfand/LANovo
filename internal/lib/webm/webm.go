// Package webm reads audio or video frames out of a WebM (Matroska) stream as it arrives: the track it
// carries, then each block's frame and timestamp. Everything else in the file is skipped.
package webm

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Element ids, from the Matroska specification.
const (
	idEBML               = 0x1A45DFA3
	idSegment            = 0x18538067
	idTracks             = 0x1654AE6B
	idTrackEntry         = 0xAE
	idTrackNumber        = 0xD7
	idCodecID            = 0x86
	idCodecPrivate       = 0x63A2
	idAudio              = 0xE1
	idSampleRate         = 0xB5
	idChannels           = 0x9F
	idCues               = 0x1C53BB6B
	idCuePoint           = 0xBB
	idCueTime            = 0xB3
	idCueTrackPositions  = 0xB7
	idCueClusterPosition = 0xF1
	idVideo              = 0xE0
	idPixelWidth         = 0xB0
	idPixelHeight        = 0xBA
	idInfo               = 0x1549A966
	idTimecodeScale      = 0x2AD7B1
	idCluster            = 0x1F43B675
	idTimecode           = 0xE7
	idSimpleBlock        = 0xA3
	idBlockGroup         = 0xA0
	idBlock              = 0xA1
)

// unknown is the size of an element that runs until its parent ends.
const unknown = -1

// Track is the audio or video track a stream carries.
type Track struct {
	Number   uint64
	Codec    string
	Private  []byte
	Rate     float64
	Channels int

	Width, Height int
}

// Frame is one block's payload and when it plays, in nanoseconds from the start.
type Frame struct {
	Track uint64
	Time  int64
	Data  []byte
	Key   bool
}

// Reader walks a stream once, front to back.
type Reader struct {
	r *bufio.Reader

	kind    byte
	scale   int64
	cluster int64
	track   *Track

	src       *counter
	segStart  int64
	cues      []cue
	clustered bool
}

type cue struct{ time, pos int64 }

type counter struct {
	r io.Reader
	n int64
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func NewReader(r io.Reader) *Reader {
	c := &counter{r: r}
	return &Reader{r: bufio.NewReaderSize(c, 64*1024), src: c, kind: 'A', scale: 1_000_000}
}

func (r *Reader) pos() int64 { return r.src.n - int64(r.r.Buffered()) }

func (r *Reader) Index() error {
	for len(r.cues) == 0 && !r.clustered {
		if _, err := r.step(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reader) Cue(at int64) (offset, time int64, ok bool) {
	for _, c := range r.cues {
		if c.time*r.scale > at {
			break
		}
		offset, time, ok = r.segStart+c.pos, c.time*r.scale, true
	}
	return offset, time, ok
}

func (r *Reader) Restart(src io.Reader, offset int64) {
	r.src.r, r.src.n = src, offset
	r.r.Reset(r.src)
}

func NewVideoReader(r io.Reader) *Reader {
	v := NewReader(r)
	v.kind = 'V'
	return v
}

// Track reads until the track header has gone past and returns the first track of the reader's kind.
func (r *Reader) Track() (Track, error) {
	for r.track == nil {
		if _, err := r.step(); err != nil {
			return Track{}, err
		}
	}
	return *r.track, nil
}

// Next is the next frame of the track, or io.EOF at the end of the stream.
func (r *Reader) Next() (Frame, error) {
	for {
		f, err := r.step()
		if err != nil {
			return Frame{}, err
		}
		if f != nil && (r.track == nil || f.Track == r.track.Number) {
			return *f, nil
		}
	}
}

// step reads one element, descending into the containers that hold what is wanted.
func (r *Reader) step() (*Frame, error) {
	id, err := r.id()
	if err != nil {
		return nil, err
	}
	size, err := r.size()
	if err != nil {
		return nil, noEOF(err)
	}

	switch id {
	case idSegment:
		r.segStart = r.pos()
		return nil, nil
	case idCluster:
		r.clustered = true
		return nil, nil
	case idBlockGroup, idInfo:
		return nil, nil
	case idCues:
		return nil, r.readCues(size)
	case idTracks:
		return nil, r.tracks(size)
	case idTimecodeScale:
		v, err := r.uint(size)
		if err == nil && v > 0 {
			r.scale = int64(v)
		}
		return nil, err
	case idTimecode:
		v, err := r.uint(size)
		r.cluster = int64(v)
		return nil, err
	case idSimpleBlock, idBlock:
		return r.block(size)
	}
	return nil, r.skip(size)
}

func (r *Reader) tracks(size int64) error {
	body, err := r.bytes(size)
	if err != nil {
		return err
	}
	return each(body, func(id uint64, b []byte) error {
		if id != idTrackEntry {
			return nil
		}
		t := Track{}
		err := each(b, func(id uint64, b []byte) error {
			switch id {
			case idTrackNumber:
				t.Number = beUint(b)
			case idCodecID:
				t.Codec = string(b)
			case idCodecPrivate:
				t.Private = append([]byte(nil), b...)
			case idAudio:
				return each(b, func(id uint64, b []byte) error {
					switch id {
					case idSampleRate:
						t.Rate = beFloat(b)
					case idChannels:
						t.Channels = int(beUint(b))
					}
					return nil
				})
			case idVideo:
				return each(b, func(id uint64, b []byte) error {
					switch id {
					case idPixelWidth:
						t.Width = int(beUint(b))
					case idPixelHeight:
						t.Height = int(beUint(b))
					}
					return nil
				})
			}
			return nil
		})
		if err == nil && r.track == nil && t.Codec != "" && t.Codec[0] == r.kind {
			r.track = &t
		}
		return err
	})
}

func (r *Reader) readCues(size int64) error {
	body, err := r.bytes(size)
	if err != nil {
		return err
	}
	return each(body, func(id uint64, b []byte) error {
		if id != idCuePoint {
			return nil
		}
		var c cue
		err := each(b, func(id uint64, b []byte) error {
			switch id {
			case idCueTime:
				c.time = int64(beUint(b))
			case idCueTrackPositions:
				return each(b, func(id uint64, b []byte) error {
					if id == idCueClusterPosition {
						c.pos = int64(beUint(b))
					}
					return nil
				})
			}
			return nil
		})
		if err == nil {
			r.cues = append(r.cues, c)
		}
		return err
	})
}

func (r *Reader) block(size int64) (*Frame, error) {
	body, err := r.bytes(size)
	if err != nil {
		return nil, err
	}
	track, n := vint(body, true)
	if n <= 0 || len(body) < n+3 {
		return nil, fmt.Errorf("webm: a block too short to have a header")
	}
	rel := int16(binary.BigEndian.Uint16(body[n:]))
	flags := body[n+2]
	if flags&0x06 != 0 {
		return nil, fmt.Errorf("webm: laced blocks are not handled")
	}
	return &Frame{
		Track: track,
		Time:  (r.cluster + int64(rel)) * r.scale,
		Data:  body[n+3:],
		Key:   flags&0x80 != 0,
	}, nil
}

func (r *Reader) id() (uint64, error) {
	first, err := r.r.ReadByte()
	if err != nil {
		return 0, err
	}
	n := width(first)
	if n == 0 || n > 4 {
		return 0, fmt.Errorf("webm: an element id of %d bytes", n)
	}
	id := uint64(first)
	for i := 1; i < n; i++ {
		b, err := r.r.ReadByte()
		if err != nil {
			return 0, noEOF(err)
		}
		id = id<<8 | uint64(b)
	}
	return id, nil
}

func (r *Reader) size() (int64, error) {
	first, err := r.r.ReadByte()
	if err != nil {
		return 0, err
	}
	n := width(first)
	if n == 0 {
		return 0, fmt.Errorf("webm: an element size with no length marker")
	}
	v := uint64(first) & (0xFF >> n)
	all := v == 0xFF>>n
	for i := 1; i < n; i++ {
		b, err := r.r.ReadByte()
		if err != nil {
			return 0, err
		}
		v = v<<8 | uint64(b)
		all = all && b == 0xFF
	}
	if all {
		return unknown, nil
	}
	return int64(v), nil
}

func (r *Reader) bytes(size int64) ([]byte, error) {
	if size < 0 || size > 16<<20 {
		return nil, fmt.Errorf("webm: an element of %d bytes", size)
	}
	b := make([]byte, size)
	_, err := io.ReadFull(r.r, b)
	return b, noEOF(err)
}

func (r *Reader) uint(size int64) (uint64, error) {
	b, err := r.bytes(size)
	return beUint(b), err
}

func (r *Reader) skip(size int64) error {
	if size == unknown {
		return nil
	}
	_, err := r.r.Discard(int(size))
	return noEOF(err)
}

// each walks the elements packed in a buffer.
func each(b []byte, fn func(id uint64, body []byte) error) error {
	for len(b) > 0 {
		id, n := vint(b, false)
		if n <= 0 {
			return fmt.Errorf("webm: a malformed element id")
		}
		b = b[n:]
		size, m := vint(b, true)
		if m <= 0 || uint64(len(b)-m) < size {
			return fmt.Errorf("webm: a malformed element size")
		}
		if err := fn(id, b[m:m+int(size)]); err != nil {
			return err
		}
		b = b[m+int(size):]
	}
	return nil
}

// vint reads a variable-length integer, with the length marker cleared for a size or kept for an id.
func vint(b []byte, clear bool) (uint64, int) {
	if len(b) == 0 {
		return 0, 0
	}
	n := width(b[0])
	if n == 0 || n > len(b) {
		return 0, 0
	}
	v := uint64(b[0])
	if clear {
		v &= 0xFF >> n
	}
	for i := 1; i < n; i++ {
		v = v<<8 | uint64(b[i])
	}
	return v, n
}

func width(first byte) int {
	for i := 0; i < 8; i++ {
		if first&(0x80>>i) != 0 {
			return i + 1
		}
	}
	return 0
}

func beUint(b []byte) uint64 {
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v
}

func beFloat(b []byte) float64 {
	switch len(b) {
	case 4:
		return float64(float32frombits(binary.BigEndian.Uint32(b)))
	case 8:
		return float64frombits(binary.BigEndian.Uint64(b))
	}
	return 0
}

func noEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return io.ErrUnexpectedEOF
	}
	return err
}
