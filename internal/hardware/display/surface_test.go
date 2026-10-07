package display

import (
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/board"
)

type helper struct {
	t      *testing.T
	ln     *net.UnixListener
	path   string
	frames chan []uint32
	files  chan *os.File
	conns  chan *net.UnixConn
}

func newHelper(t *testing.T) *helper {
	t.Helper()
	board.Set(board.Board{Name: "test", PanelWidth: 12, PanelHeight: 20, Mounted: 90})
	t.Cleanup(func() { board.Set(board.Blueberry) })
	dir, err := os.MkdirTemp("", "sfd")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	h := &helper{t: t, path: filepath.Join(dir, "s"), frames: make(chan []uint32, 16), files: make(chan *os.File, 4), conns: make(chan *net.UnixConn, 4)}
	h.listen()
	return h
}

func (h *helper) listen() {
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: h.path, Net: "unix"})
	if err != nil {
		h.t.Fatal(err)
	}
	h.ln = ln
	h.t.Cleanup(func() { ln.Close() })
	go h.serve()
}

func (h *helper) serve() {
	for {
		c, err := h.ln.AcceptUnix()
		if err != nil {
			return
		}
		h.conns <- c
		go h.conn(c)
	}
}

func (h *helper) conn(c *net.UnixConn) {
	defer c.Close()
	for {
		hdr := make([]byte, 8)
		if _, err := io.ReadFull(c, hdr); err != nil {
			return
		}
		body := make([]byte, binary.LittleEndian.Uint32(hdr[4:]))
		if _, err := io.ReadFull(c, body); err != nil {
			return
		}
		w := make([]uint32, len(body)/4)
		for i := range w {
			w[i] = binary.LittleEndian.Uint32(body[4*i:])
		}
		switch binary.LittleEndian.Uint32(hdr) {
		case 1:
			send(c, 1, nil, 5, 12, 20)
		case 2:
			f, _ := os.CreateTemp(h.t.TempDir(), "ui")
			f.Truncate(int64(w[3] * w[4] * 4))
			h.files <- f
			send(c, 2, syscall.UnixRights(int(f.Fd())), w[0], 0, w[3]*4)
		case 3:
			h.frames <- w
			send(c, 3, nil, w[0], w[1], 0)
		}
	}
}

func send(c *net.UnixConn, op uint32, oob []byte, words ...uint32) {
	b := make([]byte, 8+4*len(words))
	binary.LittleEndian.PutUint32(b, op)
	binary.LittleEndian.PutUint32(b[4:], uint32(4*len(words)))
	for i, w := range words {
		binary.LittleEndian.PutUint32(b[8+4*i:], w)
	}
	c.WriteMsgUnix(b, oob, nil)
}

func TestDrawingLandsInTheHelpersBuffer(t *testing.T) {
	h := newHelper(t)
	p, err := OpenSurface(h.path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	file := <-h.files

	if p.Width != 20 || p.Height != 12 {
		t.Fatalf("viewed %dx%d, want the portrait panel turned to 20x12", p.Width, p.Height)
	}
	p.Set(0, 0, 9, 8, 7)
	x, y := p.rot.Project(p.fbW, p.fbH, 0, 0)
	got := make([]byte, 4)
	file.ReadAt(got, int64(y*p.stride+x*4))
	if got[0] != 9 || got[1] != 8 || got[2] != 7 || got[3] != 0xff {
		t.Errorf("helper sees %v", got)
	}
}

func TestAFlipPostsTheClipInNativeCoordinates(t *testing.T) {
	h := newHelper(t)
	p, err := OpenSurface(h.path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	<-h.files

	p.Clip(Rect{X: 2, Y: 3, W: 4, H: 5})
	if err := p.Flip(); err != nil {
		t.Fatal(err)
	}
	f := <-h.frames
	x0, y0, x1, y1 := p.fbRect(Rect{X: 2, Y: 3, W: 4, H: 5})
	if f[2] != 1 || f[3] != uint32(x0) || f[4] != uint32(y0) || f[5] != uint32(x1-x0) || f[6] != uint32(y1-y0) {
		t.Errorf("frame %v, want rect %d,%d %dx%d", f, x0, y0, x1-x0, y1-y0)
	}
}

func TestLosingTheHelperReconnectsWithTheFrame(t *testing.T) {
	h := newHelper(t)
	p, err := OpenSurface(h.path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	<-h.files
	p.Set(1, 1, 50, 60, 70)

	h.ln.Close()
	(<-h.conns).Close()
	h.listen()

	if err := p.Flip(); err != nil {
		t.Fatal(err)
	}
	file := <-h.files
	f := <-h.frames
	if f[3] != 0 || f[4] != 0 || f[5] != 12 || f[6] != 20 {
		t.Errorf("after reconnecting the frame was %v, want the whole panel", f)
	}
	x, y := p.rot.Project(p.fbW, p.fbH, 1, 1)
	got := make([]byte, 3)
	file.ReadAt(got, int64(y*p.stride+x*4))
	if got[0] != 50 || got[1] != 60 || got[2] != 70 {
		t.Errorf("the new buffer holds %v, want what was drawn before", got)
	}
}
