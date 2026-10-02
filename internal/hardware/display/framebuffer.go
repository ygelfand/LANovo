// Package display owns the panel.
//
// The panel is natively portrait, 1200x1920. Callers work in what a person sees, origin top-left,
// and Orientation reconciles that with the framebuffer — for drawing and for touch alike.
package display

import (
	"fmt"
	"os"
	"sync"
	"syscall"

	"github.com/ygelfand/LANovo/internal/lib/hook"
	"github.com/ygelfand/LANovo/internal/lib/surface"
	"time"
	"unsafe"
)

// The framebuffer ioctls we need. MDSS video-mode panels reject write(2) with ENODEV, so drawing
// means mmap, and the panel only stays lit while this process holds the fd open.
const (
	iocGetVScreenInfo = 0x4600
	iocGetFScreenInfo = 0x4602
	iocPanDisplay     = 0x4606
	iocBlank          = 0x4611
	iocWaitForVSync   = 0x4620

	blankUnblank = 0
)

type bitfield struct{ Offset, Length, MSBRight uint32 }

// varInfo is fb_var_screeninfo. Its size is not encoded in the ioctl number, but the layout still
// has to match the kernel's exactly or the fields read as nonsense.
type varInfo struct {
	Xres, Yres               uint32
	XresVirtual, YresVirtual uint32
	Xoffset, Yoffset         uint32
	BitsPerPixel, Grayscale  uint32
	Red, Green, Blue, Transp bitfield
	Nonstd, Activate         uint32
	Height, Width            uint32
	AccelFlags               uint32
	Pixclock                 uint32
	LeftMargin, RightMargin  uint32
	UpperMargin, LowerMargin uint32
	HsyncLen, VsyncLen       uint32
	Sync, Vmode, Rotate      uint32
	Colorspace               uint32
	Reserved                 [4]uint32
}

type fixInfo struct {
	ID                            [16]byte
	SmemStart                     uint32
	SmemLen                       uint32
	Type, TypeAux, Visual         uint32
	Xpanstep, Ypanstep, Ywrapstep uint16
	_                             uint16
	LineLength                    uint32
	MmioStart                     uint32
	MmioLen, Accel                uint32
	Capabilities                  uint16
	Reserved                      [2]uint16
}

// Panel is the screen, held open.
type Panel struct {
	f    *os.File
	mem  []byte
	var_ varInfo

	stride int
	fbW    int // framebuffer width, portrait
	fbH    int // framebuffer height, portrait

	// rot is how the picture sits on the panel. Everything above here draws in viewed
	// coordinates and this is what reconciles them.
	rot Orientation

	// Width and Height are what a person sees, at the current rotation.
	Width  int
	Height int

	// back is the byte offset of the buffer drawn into, front the one on screen.
	back, front int
	doubled     bool

	// clip bounds what may be painted, in viewed coordinates. Empty means the whole panel. It is
	// how a frame that changed one row costs one row: the drawing code carries on describing the
	// whole screen and everything outside the clip is discarded before it reaches memory.
	clip Rect

	fbmem []byte

	// BlankErr is what the unblank said. Reported so a dark panel is distinguishable from a
	// drawing mistake.
	BlankErr error

	surf  *surface.Client
	layer *surface.Layer
	sock  string
	seq   uint32

	Dropped hook.Hook[error]
}

// Open takes the panel. SurfaceFlinger has to be stopped first or it draws over us.
func Open(path string) (*Panel, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	p := &Panel{f: f}
	// The driver powers the panel down once SurfaceFlinger lets go, and a sysfs unblank only holds
	// while someone has the fd. Not fatal: some kernels refuse it on an already-lit panel.
	p.BlankErr = p.ioctlValue(iocBlank, blankUnblank)

	if err := p.ioctl(iocGetVScreenInfo, unsafe.Pointer(&p.var_)); err != nil {
		f.Close()
		return nil, fmt.Errorf("fb var info: %w", err)
	}
	var fx fixInfo
	if err := p.ioctl(iocGetFScreenInfo, unsafe.Pointer(&fx)); err != nil {
		f.Close()
		return nil, fmt.Errorf("fb fix info: %w", err)
	}
	if p.var_.BitsPerPixel != 32 {
		f.Close()
		return nil, fmt.Errorf("panel is %d bpp, this expects 32", p.var_.BitsPerPixel)
	}

	p.stride = int(fx.LineLength)
	p.fbW, p.fbH = int(p.var_.Xres), int(p.var_.Yres)
	p.rot = Mounted()
	p.Width, p.Height = p.rot.Size(p.fbW, p.fbH)

	size := int(fx.SmemLen)
	if size == 0 {
		size = p.stride * int(p.var_.YresVirtual)
	}
	mem, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("mmap %d bytes: %w", size, err)
	}
	p.mem, p.fbmem = mem, mem

	second := p.stride * p.fbH
	p.doubled = int(p.var_.YresVirtual) >= 2*p.fbH && second+p.stride*p.fbH <= len(mem)

	// Draw into whichever buffer is not on screen. This panel is video mode, so the MDP fetches
	// the live buffer continuously and drawing into it tears.
	p.front = 0
	if p.doubled && p.var_.Yoffset >= uint32(p.fbH) {
		p.front = second
	}
	p.back = 0
	if p.doubled && p.front == 0 {
		p.back = second
	}
	return p, nil
}

// Close puts the mappings down, leaving on screen whatever was drawn.
func (p *Panel) Close() error {
	if p.surf != nil {
		p.mem = nil
		return p.surf.Close()
	}
	if p.fbmem != nil {
		syscall.Munmap(p.fbmem)
		p.fbmem = nil
	}
	p.mem = nil
	return p.f.Close()
}

func (p *Panel) Native() (w, h int) { return p.fbW, p.fbH }

// Doubled reports whether Flip is a real page flip.
func (p *Panel) Doubled() bool { return p.doubled }

// Info describes the panel, for logs.
func (p *Panel) Info() string {
	if p.surf != nil {
		return fmt.Sprintf("%dx%d %s (native %dx%d, SurfaceFlinger through %s)", p.Width, p.Height, p.rot, p.fbW, p.fbH, p.sock)
	}
	s := fmt.Sprintf("%dx%d %s (fb %dx%d, stride %d, %d bpp, double-buffered=%v, panned)",
		p.Width, p.Height, p.rot, p.fbW, p.fbH, p.stride, p.var_.BitsPerPixel, p.doubled)
	if p.BlankErr != nil {
		s += fmt.Sprintf(", unblank: %v", p.BlankErr)
	}
	return s
}

// Orientation is how the picture currently sits on the panel.
func (p *Panel) Orientation() Orientation { return p.rot }

// Turn changes which way the picture faces. What is on the panel is left alone: whatever draws
// next does so at the new rotation.
func (p *Panel) Turn(rot Orientation) {
	p.rot = rot
	p.Width, p.Height = rot.Size(p.fbW, p.fbH)
}

// Clip bounds what the next drawing may paint. An empty rectangle means all of it.
func (p *Panel) Clip(r Rect) { p.clip = r }

// Clipped is what is currently allowed to be painted.
func (p *Panel) Clipped() Rect { return p.clip }

// inClip reports whether a point may be painted.
func (p *Panel) inClip(vx, vy int) bool {
	c := p.clip
	if c.W <= 0 || c.H <= 0 {
		return true
	}
	return vx >= c.X && vy >= c.Y && vx < c.X+c.W && vy < c.Y+c.H
}

// Set paints one pixel in viewed coordinates, rotating into the framebuffer.
func (p *Panel) Set(vx, vy int, r, g, b byte) {
	if vx < 0 || vy < 0 || vx >= p.Width || vy >= p.Height {
		return
	}
	if !p.inClip(vx, vy) {
		return
	}
	x, y := p.rot.Project(p.fbW, p.fbH, vx, vy)
	at := p.back + y*p.stride + x*4
	if at+4 > len(p.mem) {
		return
	}
	// RGBA8888: red first (driver reports R off=0, G=8, B=16, A=24).
	p.mem[at], p.mem[at+1], p.mem[at+2], p.mem[at+3] = r, g, b, 0xff
}

func (p *Panel) Over(vx, vy int, r, g, b, a byte) {
	if vx < 0 || vy < 0 || vx >= p.Width || vy >= p.Height || !p.inClip(vx, vy) {
		return
	}
	x, y := p.rot.Project(p.fbW, p.fbH, vx, vy)
	at := p.back + y*p.stride + x*4
	if at+4 > len(p.mem) {
		return
	}
	d := p.mem[at : at+4 : at+4]
	keep := 255 - int(a)
	d[0] = byte(min(int(r)+(int(d[0])*keep+127)/255, 255))
	d[1] = byte(min(int(g)+(int(d[1])*keep+127)/255, 255))
	d[2] = byte(min(int(b)+(int(d[2])*keep+127)/255, 255))
	d[3] = byte(min(int(a)+(int(d[3])*keep+127)/255, 255))
}

// Fill paints the whole back buffer one color.
// FillRect paints a rectangle given in viewed coordinates.
//
// A quarter turn maps a rectangle to a rectangle, so the corners are projected once and the span
// between them filled a row at a time. Going through Set instead costs a Project and a bounds
// check per pixel, which on a full screen is 2.3 million of each.
func (p *Panel) FillRect(vx, vy, w, h int, r, g, b byte) {
	x0, y0 := max(vx, 0), max(vy, 0)
	x1, y1 := min(vx+w, p.Width)-1, min(vy+h, p.Height)-1

	if c := p.clip; c.W > 0 && c.H > 0 {
		x0, y0 = max(x0, c.X), max(y0, c.Y)
		x1, y1 = min(x1, c.X+c.W-1), min(y1, c.Y+c.H-1)
	}
	if x1 < x0 || y1 < y0 {
		return
	}

	fx0, fy0 := p.rot.Project(p.fbW, p.fbH, x0, y0)
	fx1, fy1 := p.rot.Project(p.fbW, p.fbH, x1, y1)
	if fx0 > fx1 {
		fx0, fx1 = fx1, fx0
	}
	if fy0 > fy1 {
		fy0, fy1 = fy1, fy0
	}

	// The first row is written a pixel at a time and every other row is copied from it. Copy is a
	// memmove, which stores a vector at a time: framebuffer memory is write combining, where four
	// byte stores per pixel is close to the worst thing that can be done to it.
	first := p.span(fy0, fx0, fx1)
	if first == nil {
		return
	}

	for i := 0; i+4 <= len(first); i += 4 {
		first[i], first[i+1], first[i+2], first[i+3] = r, g, b, 0xff
	}

	for y := fy0 + 1; y <= fy1; y++ {
		if row := p.span(y, fx0, fx1); row != nil {
			copy(row, first)
		}
	}
}

// span is one row of the back buffer between two columns, inclusive.
func (p *Panel) span(y, x0, x1 int) []byte {
	at := p.back + y*p.stride + x0*4
	end := p.back + y*p.stride + (x1+1)*4

	if at < 0 || end > len(p.mem) || end <= at {
		return nil
	}
	return p.mem[at:end]
}

func (p *Panel) Fill(r, g, b byte) { p.FillRect(0, 0, p.Width, p.Height, r, g, b) }

// DrawRGBA paints an RGBA image with its top-left at vx, vy in viewed coordinates, each pixel
// scale times across and down, inside clip as well as the panel's own clip.
func (p *Panel) DrawRGBA(vx, vy int, pix []byte, stride, w, h, scale int, clip Rect) {
	scale = max(scale, 1)
	x0, y0 := max(vx, 0), max(vy, 0)
	x1, y1 := min(vx+w*scale, p.Width), min(vy+h*scale, p.Height)
	for _, c := range []Rect{clip, p.clip} {
		if c.W > 0 && c.H > 0 {
			x0, y0 = max(x0, c.X), max(y0, c.Y)
			x1, y1 = min(x1, c.X+c.W), min(y1, c.Y+c.H)
		}
	}
	if x1 <= x0 || y1 <= y0 {
		return
	}
	if p.rot == Rotate90 || p.rot == Rotate270 {
		p.drawTurned(vx, vy, pix, stride, scale, x0, y0, x1, y1)
		return
	}

	n := (x1 - x0) * 4
	held, row := scratch(n)
	defer rows.Put(held)

	sx0, phase0 := (x0-vx)/scale, (x0-vx)%scale
	sy, phy := (y0-vy)/scale, (y0-vy)%scale
	built := -1
	for y := y0; y < y1; y++ {
		if sy != built {
			src := pix[sy*stride:]
			s, ph := sx0*4, phase0
			for d := 0; d < n; d += 4 {
				e := d
				if p.rot == Rotate180 {
					e = n - 4 - d
				}
				row[e], row[e+1], row[e+2], row[e+3] = src[s], src[s+1], src[s+2], 0xff
				if ph++; ph == scale {
					ph, s = 0, s+4
				}
			}
			built = sy
		}
		switch p.rot {
		case Rotate0:
			if dst := p.span(y, x0, x1-1); dst != nil {
				copy(dst, row)
			}
		case Rotate180:
			if dst := p.span(p.fbH-1-y, p.fbW-x1, p.fbW-1-x0); dst != nil {
				copy(dst, row)
			}
		}
		if phy++; phy == scale {
			phy, sy = 0, sy+1
		}
	}
}

var rows = sync.Pool{New: func() any { return new([]byte) }}

func scratch(n int) (*[]byte, []byte) {
	b := rows.Get().(*[]byte)
	if cap(*b) < n {
		*b = make([]byte, n)
	}
	return b, (*b)[:n]
}

func (p *Panel) drawTurned(vx, vy int, pix []byte, stride, scale, x0, y0, x1, y1 int) {
	n := (y1 - y0) * 4
	held, row := scratch(n)
	defer rows.Put(held)

	sx, phx := (x0-vx)/scale, (x0-vx)%scale
	sy0, phase0 := (y0-vy)/scale, (y0-vy)%scale
	built := -1
	for x := x0; x < x1; x++ {
		if sx != built {
			s, ph := sy0*stride+sx*4, phase0
			for d := 0; d < n; d += 4 {
				e := d
				if p.rot == Rotate270 {
					e = n - 4 - d
				}
				row[e], row[e+1], row[e+2], row[e+3] = pix[s], pix[s+1], pix[s+2], 0xff
				if ph++; ph == scale {
					ph, s = 0, s+stride
				}
			}
			built = sx
		}
		var dst []byte
		if p.rot == Rotate90 {
			dst = p.span(p.fbH-1-x, y0, y1-1)
		} else {
			dst = p.span(x, p.fbW-y1, p.fbW-1-y0)
		}
		if dst != nil {
			copy(dst, row)
		}
		if phx++; phx == scale {
			phx, sx = 0, sx+1
		}
	}
}

// Flip shows what was drawn and turns the buffers round.
func (p *Panel) Flip() error {
	if p.surf != nil {
		return p.flipSurface()
	}
	if p.doubled {
		p.var_.Yoffset = uint32(p.back / p.stride)
	} else {
		p.var_.Yoffset = 0
	}
	p.var_.Activate = 0 // FB_ACTIVATE_NOW
	if err := p.ioctl(iocPanDisplay, unsafe.Pointer(&p.var_)); err != nil {
		return fmt.Errorf("pan: %w", err)
	}
	if p.doubled {
		p.back, p.front = p.front, p.back
	}
	return nil
}

// WaitVSync blocks until the next vertical sync, for pacing an animation without spinning.
func (p *Panel) WaitVSync() error {
	if p.surf != nil {
		time.Sleep(time.Second / 60)
		return nil
	}
	var zero uint32
	return p.ioctl(iocWaitForVSync, unsafe.Pointer(&zero))
}

func (p *Panel) ioctl(req uintptr, arg unsafe.Pointer) error {
	if p.f == nil {
		return syscall.ENODEV
	}
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, p.f.Fd(), req, uintptr(arg)); e != 0 {
		return e
	}
	return nil
}

// ioctlValue is for the ioctls that take their argument by value rather than by pointer, which is
// FBIOBLANK. Casting an integer through unsafe.Pointer would work and would also be a lie.
func (p *Panel) ioctlValue(req, val uintptr) error {
	if p.f == nil {
		return syscall.ENODEV
	}
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, p.f.Fd(), req, val); e != 0 {
		return e
	}
	return nil
}

// ClearRect makes a rectangle given in viewed coordinates transparent, so what is under the
// drawing shows through it.
func (p *Panel) ClearRect(vx, vy, w, h int) {
	x0, y0 := max(vx, 0), max(vy, 0)
	x1, y1 := min(vx+w, p.Width)-1, min(vy+h, p.Height)-1
	if c := p.clip; c.W > 0 && c.H > 0 {
		x0, y0 = max(x0, c.X), max(y0, c.Y)
		x1, y1 = min(x1, c.X+c.W-1), min(y1, c.Y+c.H-1)
	}
	if x1 < x0 || y1 < y0 {
		return
	}
	fx0, fy0 := p.rot.Project(p.fbW, p.fbH, x0, y0)
	fx1, fy1 := p.rot.Project(p.fbW, p.fbH, x1, y1)
	if fx0 > fx1 {
		fx0, fx1 = fx1, fx0
	}
	if fy0 > fy1 {
		fy0, fy1 = fy1, fy0
	}
	for y := fy0; y <= fy1; y++ {
		clear(p.span(y, fx0, fx1))
	}
}

// Shade covers a rectangle in black that runs from one opacity at its top to another at its
// bottom, premultiplied, for what sits over something showing through.
func (p *Panel) Shade(vx, vy, w, h int, top, bottom byte) {
	for y := max(vy, 0); y < min(vy+h, p.Height); y++ {
		a := byte(int(top) + (int(bottom)-int(top))*(y-vy)/max(h-1, 1))
		for x := max(vx, 0); x < min(vx+w, p.Width); x++ {
			if !p.inClip(x, y) {
				continue
			}
			fx, fy := p.rot.Project(p.fbW, p.fbH, x, y)
			at := p.back + fy*p.stride + fx*4
			if at+4 <= len(p.mem) {
				p.mem[at], p.mem[at+1], p.mem[at+2], p.mem[at+3] = 0, 0, 0, a
			}
		}
	}
}

func (p *Panel) fbRect(r Rect) (x0, y0, x1, y1 int) {
	if r.W <= 0 || r.H <= 0 {
		return 0, 0, p.fbW, p.fbH
	}
	vx0, vy0 := max(r.X, 0), max(r.Y, 0)
	vx1, vy1 := min(r.X+r.W, p.Width), min(r.Y+r.H, p.Height)
	if vx1 <= vx0 || vy1 <= vy0 {
		return 0, 0, 0, 0
	}
	ax, ay := p.rot.Project(p.fbW, p.fbH, vx0, vy0)
	bx, by := p.rot.Project(p.fbW, p.fbH, vx1-1, vy1-1)
	return min(ax, bx), min(ay, by), max(ax, bx) + 1, max(ay, by) + 1
}

// Front reads a pixel of what is on screen rather than what is being drawn, which is what a
// screenshot wants: the buffer being drawn into holds the frame before last.
func (p *Panel) Front(vx, vy int) (r, g, b byte) {
	mem, at := p.fbmem, p.front
	if p.surf != nil {
		mem, at = p.mem, 0
	}
	return p.read(mem, at, vx, vy)
}

// Snapshot copies what is on the panel, in viewed coordinates, as RGB triples read row by row.
//
// The buffer is resolved once rather than per pixel. Front asks which one is showing every time it
// is called, so a flip partway through a read returned the frames either side of it blended down
// the screen — a page transition came back with its header at one offset and its rows at another.
//
// Still to be called where nothing is flipping: this closes the window to the width of one memcpy
// rather than one screen read, and the render loop is what closes it altogether.
func (p *Panel) Snapshot() (pixels []byte, w, h int) {
	mem, base := p.fbmem, p.front
	if p.surf != nil {
		mem, base = p.mem, 0
	}

	w, h = p.Width, p.Height
	pixels = make([]byte, w*h*3)

	for y := range h {
		for x := range w {
			r, g, b := p.read(mem, base, x, y)
			at := (y*w + x) * 3
			pixels[at], pixels[at+1], pixels[at+2] = r, g, b
		}
	}
	return pixels, w, h
}

// At reads a pixel back, in viewed coordinates. Antialiasing needs to know what it is blending
// with, and the layer buffers are ordinary cached memory.
func (p *Panel) At(vx, vy int) (r, g, b byte) { return p.read(p.mem, p.back, vx, vy) }

func (p *Panel) read(mem []byte, base, vx, vy int) (r, g, b byte) {
	if vx < 0 || vy < 0 || vx >= p.Width || vy >= p.Height {
		return 0, 0, 0
	}

	x, y := p.rot.Project(p.fbW, p.fbH, vx, vy)
	at := base + y*p.stride + x*4
	if at+4 > len(mem) {
		return 0, 0, 0
	}
	return mem[at], mem[at+1], mem[at+2]
}
