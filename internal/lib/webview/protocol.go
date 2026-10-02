// Package webview is a client for Remote WebView Server (github.com/strange-v/RemoteWebViewServer, MIT).
package webview

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

const version = 1

const (
	msgFrame      = 1
	msgTouch      = 2
	msgFrameStats = 3
	msgOpenURL    = 4
	msgKeepalive  = 5
	msgCurrentURL = 6
)

type Encoding uint8

const (
	PNG    Encoding = 1
	JPEG   Encoding = 2
	RAW565 Encoding = 3
)

// A frame can span several messages; only the last carries FlagLastOfFrame.
const (
	FlagLastOfFrame = 1 << 0
	FlagFullFrame   = 1 << 1
)

type TouchKind uint8

const (
	Down TouchKind = 1
	Move TouchKind = 2
	Up   TouchKind = 3
)

type Tile struct {
	X, Y, W, H int
	Data       []byte
}

type Frame struct {
	ID       uint32
	Encoding Encoding
	Flags    uint16
	Tiles    []Tile
}

type Options struct {
	ID               string
	Width, Height    int
	TileSize         int
	FullFrameTiles   int
	FullFrameArea    float64
	FullFrameEvery   int
	EveryNth         int
	MinFrameInterval int
	JPEGQuality      int
	MaxMessageBytes  int
}

func URI(server string, opts Options) (string, error) {
	u, err := url.Parse(server)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.Host == "" {
		u, err = url.Parse("ws://" + server)
	}
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("webview: server %q is not host:port or a ws:// URL", server)
	}
	if u.Path == "" {
		u.Path = "/"
	}
	q := u.Query()
	q.Set("id", opts.ID)
	q.Set("w", strconv.Itoa(opts.Width))
	q.Set("h", strconv.Itoa(opts.Height))
	for k, v := range map[string]int{
		"ts": opts.TileSize, "fftc": opts.FullFrameTiles, "ffe": opts.FullFrameEvery,
		"enf": opts.EveryNth, "mfi": opts.MinFrameInterval, "q": opts.JPEGQuality, "mbpm": opts.MaxMessageBytes,
	} {
		if v > 0 {
			q.Set(k, strconv.Itoa(v))
		}
	}
	if opts.FullFrameArea > 0 {
		q.Set("ffat", strconv.FormatFloat(opts.FullFrameArea, 'f', -1, 64))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

var errShort = errors.New("webview: message cut short")

func ParseFrame(b []byte) (Frame, error) {
	if len(b) < 11 {
		return Frame{}, errShort
	}
	if b[0] != msgFrame || b[1] != version {
		return Frame{}, fmt.Errorf("webview: message type %d version %d, want a version %d frame", b[0], b[1], version)
	}
	f := Frame{
		ID:       binary.LittleEndian.Uint32(b[2:]),
		Encoding: Encoding(b[6]),
		Flags:    binary.LittleEndian.Uint16(b[9:]),
	}
	n := int(binary.LittleEndian.Uint16(b[7:]))
	at := 11
	for range n {
		if at+12 > len(b) {
			return Frame{}, errShort
		}
		t := Tile{
			X: int(binary.LittleEndian.Uint16(b[at:])),
			Y: int(binary.LittleEndian.Uint16(b[at+2:])),
			W: int(binary.LittleEndian.Uint16(b[at+4:])),
			H: int(binary.LittleEndian.Uint16(b[at+6:])),
		}
		size := uint64(binary.LittleEndian.Uint32(b[at+8:]))
		at += 12
		if uint64(at)+size > uint64(len(b)) {
			return Frame{}, errShort
		}
		t.Data = b[at : at+int(size)]
		at += int(size)
		f.Tiles = append(f.Tiles, t)
	}
	return f, nil
}

func ParseCurrentURL(b []byte) (string, error) {
	if len(b) < 6 || b[0] != msgCurrentURL {
		return "", errShort
	}
	n := uint64(binary.LittleEndian.Uint32(b[2:]))
	if 6+n > uint64(len(b)) {
		return "", errShort
	}
	return string(b[6 : 6+int(n)]), nil
}

func touch(kind TouchKind, pointer uint8, x, y int) []byte {
	b := []byte{msgTouch, version, byte(kind), pointer, 0, 0, 0, 0}
	binary.LittleEndian.PutUint16(b[4:], clampU16(x))
	binary.LittleEndian.PutUint16(b[6:], clampU16(y))
	return b
}

func openURL(u string) []byte {
	b := make([]byte, 8, 8+len(u))
	b[0], b[1] = msgOpenURL, version
	binary.LittleEndian.PutUint32(b[4:], uint32(len(u)))
	return append(b, u...)
}

func frameStats(avgMillis, bytes uint32) []byte {
	b := []byte{msgFrameStats, version, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(b[2:], avgMillis)
	binary.LittleEndian.PutUint32(b[6:], bytes)
	return b
}

func keepalive() []byte { return []byte{msgKeepalive, version} }

func clampU16(v int) uint16 { return uint16(max(0, min(v, 0xffff))) }
