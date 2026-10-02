// Package mtkcamera streams H.264 from lanovo-camera, the helper that runs the vendor camera stack into
// hardware encoders. One client at a time; closing the stream stops the camera.
package mtkcamera

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/ygelfand/LANovo/internal/layout"
)

const (
	magic   = 0x4d434e4c
	version = 5

	frameKey    = 1
	frameConfig = 2
	frameSub    = 4
	frameStill  = 8

	askStill  = 'S'
	askParams = 'P'
	maxParams = 4096

	maxFrame   = 16 << 20
	noTurn     = 0xffffffff
	turnMirror = 0x100
)

var errs = map[uint32]string{
	1: "bad arguments",
	2: "the camera refused",
	3: "the encoder refused",
	4: "the helper speaks another protocol version",
}

type Config struct {
	Width, Height, FPS, Bitrate, Keyframe int

	SubWidth, SubHeight, SubBitrate int

	// Params is vendor camera parameters as key=value pairs joined by semicolons.
	Params string

	// Turn is quarter turns clockwise drawn on the GPU into the encoders, negative for none.
	Turn int

	Mirror bool
}

type Frame struct {
	Data   []byte
	PTS    time.Duration
	Key    bool
	Config bool
	Sub    bool

	Still         bool
	Width, Height int
}

type Stream struct {
	c   net.Conn
	cfg Config
}

var socket = layout.CameraSocket

// Open starts the camera and encoder.
func Open(cfg Config) (*Stream, error) {
	c, err := net.DialTimeout("unix", socket, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("mtkcamera: %w", err)
	}
	req := []uint32{magic, version, uint32(cfg.Width), uint32(cfg.Height), uint32(cfg.FPS),
		uint32(cfg.Bitrate), uint32(cfg.Keyframe), uint32(cfg.SubWidth), uint32(cfg.SubHeight), uint32(cfg.SubBitrate),
		turnWord(cfg.Turn, cfg.Mirror)}
	if len(cfg.Params) > maxParams {
		c.Close()
		return nil, fmt.Errorf("mtkcamera: %d bytes of parameters", len(cfg.Params))
	}
	req = append(req, uint32(len(cfg.Params)))
	if err := binary.Write(c, binary.LittleEndian, req); err != nil {
		c.Close()
		return nil, fmt.Errorf("mtkcamera: %w", err)
	}
	if _, err := io.WriteString(c, cfg.Params); err != nil {
		c.Close()
		return nil, fmt.Errorf("mtkcamera: %w", err)
	}
	c.SetReadDeadline(time.Now().Add(15 * time.Second))
	var reply [4]uint32
	if err := binary.Read(c, binary.LittleEndian, &reply); err != nil {
		c.Close()
		return nil, fmt.Errorf("mtkcamera: opening %dx%d@%d: %w", cfg.Width, cfg.Height, cfg.FPS, err)
	}
	if reply[0] != 0 {
		c.Close()
		if why, ok := errs[reply[0]]; ok {
			return nil, fmt.Errorf("mtkcamera: opening %dx%d@%d: %s", cfg.Width, cfg.Height, cfg.FPS, why)
		}
		return nil, fmt.Errorf("mtkcamera: opening %dx%d@%d: status %d", cfg.Width, cfg.Height, cfg.FPS, reply[0])
	}
	c.SetReadDeadline(time.Time{})
	return &Stream{c: c, cfg: cfg}, nil
}

func (s *Stream) Config() Config { return s.cfg }

func turnWord(q int, mirror bool) uint32 {
	if q < 0 {
		return noTurn
	}
	w := uint32(q & 3)
	if mirror {
		w |= turnMirror
	}
	return w
}

// Next blocks until the encoder has another frame, or within passes.
func (s *Stream) Next(within time.Duration) (Frame, error) {
	if within > 0 {
		s.c.SetReadDeadline(time.Now().Add(within))
	}
	var head [4]uint32
	if err := binary.Read(s.c, binary.LittleEndian, &head); err != nil {
		return Frame{}, err
	}
	size := head[0]
	if size > maxFrame {
		return Frame{}, fmt.Errorf("mtkcamera: a %d byte frame", size)
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(s.c, data); err != nil {
		return Frame{}, err
	}
	if head[1]&frameStill != 0 {
		return Frame{Data: data, Still: true, Width: int(head[2]), Height: int(head[3])}, nil
	}
	pts := uint64(head[2]) | uint64(head[3])<<32
	return Frame{
		Data:   data,
		PTS:    time.Duration(pts) * time.Microsecond,
		Key:    head[1]&frameKey != 0 || idr(data),
		Config: head[1]&frameConfig != 0,
		Sub:    head[1]&frameSub != 0,
	}, nil
}

const idrScan = 4096

const nalIDR = 5

func idr(b []byte) bool {
	b = b[:min(len(b), idrScan)]
	for i := 0; i+3 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 && b[i+3]&0x1f == nalIDR {
			return true
		}
	}
	return false
}

// AskStill asks for an RGBA picture of the main stream, which arrives among the frames, empty on failure.
func (s *Stream) AskStill() error {
	_, err := s.c.Write([]byte{askStill})
	return err
}

// SetParams changes vendor camera parameters on the running camera.
func (s *Stream) SetParams(params string) error {
	if len(params) > maxParams {
		return fmt.Errorf("mtkcamera: %d bytes of parameters", len(params))
	}
	msg := make([]byte, 5+len(params))
	msg[0] = askParams
	binary.LittleEndian.PutUint32(msg[1:], uint32(len(params)))
	copy(msg[5:], params)
	_, err := s.c.Write(msg)
	return err
}

func (s *Stream) Close() error { return s.c.Close() }

// Timeout reports whether err is Next running out of time rather than the stream ending.
func Timeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
