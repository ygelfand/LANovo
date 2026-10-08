package alsa

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"strconv"
	"syscall"
	"unsafe"
)

// The kernel's unsigned long: 8 bytes on arm64, 4 on a 32-bit kernel.
const longSize = strconv.IntSize / 8

// struct snd_pcm_hw_params: only fifo_size is word sized.
const (
	maskOff     = 4
	maskSize    = 32
	intervalOff = maskOff + 8*maskSize // 260
	intervalLen = 12
	rmaskOff    = intervalOff + 21*intervalLen // 512
	infoOff     = rmaskOff + 8

	hwParamsSize = rmaskOff + 6*4 + longSize + 64 // 608 on arm64, 604 on 32-bit
)

// Masks are 0..2, intervals 8..19.
const (
	paramAccess    = 0
	paramFormat    = 1
	paramSubformat = 2
	firstMask      = paramAccess
	lastMask       = paramSubformat

	paramSampleBits = 8
	paramFrameBits  = 9
	paramChannels   = 10
	paramRate       = 11
	paramPeriodSize = 13
	paramPeriods    = 15
	firstInterval   = paramSampleBits
	lastInterval    = 19
)

const (
	accessRWInterleaved = 3
	subformatStd        = 0

	FormatS16LE   = 2
	FormatS32LE   = 10
	FormatS24_3LE = 32
)

// Same encoding as asm-generic/ioctl.h.
func ioc(dir, typ, nr, size uintptr) uintptr {
	return dir<<30 | size<<16 | typ<<8 | nr
}

var (
	ioctlHwParams = ioc(3, 'A', 0x11, hwParamsSize)
	ioctlPrepare  = ioc(0, 'A', 0x40, 0)
	ioctlStart    = ioc(0, 'A', 0x42, 0)
	ioctlDrop     = ioc(0, 'A', 0x43, 0)
	ioctlReadi    = ioc(2, 'A', 0x51, xferiSize)
)

type hwParams [hwParamsSize]byte

func (p *hwParams) set(off int, v uint32) { binary.LittleEndian.PutUint32(p[off:], v) }

func (p *hwParams) init() {
	for i := range p {
		p[i] = 0
	}
	for m := firstMask; m <= lastMask; m++ {
		off := maskOff + m*maskSize
		p.set(off, ^uint32(0))
		p.set(off+4, ^uint32(0))
	}
	for n := firstInterval; n <= lastInterval; n++ {
		off := intervalOff + (n-firstInterval)*intervalLen
		p.set(off, 0)
		p.set(off+4, ^uint32(0))
	}
	p.set(rmaskOff, ^uint32(0))
	p.set(infoOff, ^uint32(0))
}

func (p *hwParams) setMask(param, bit int) {
	off := maskOff + param*maskSize
	for i := 0; i < maskSize/4; i++ {
		p.set(off+i*4, 0)
	}
	p.set(off+(bit>>5)*4, 1<<uint(bit&31))
}

// Bit 2 of the flags word is "integer".
func (p *hwParams) setInterval(param int, v uint32) {
	off := intervalOff + (param-firstInterval)*intervalLen
	p.set(off, v)
	p.set(off+4, v)
	p.set(off+8, 1<<2)
}

func (p *hwParams) interval(param int) uint32 {
	off := intervalOff + (param-firstInterval)*intervalLen
	return binary.LittleEndian.Uint32(p[off:])
}

// struct snd_xferi: a signed long, a pointer and an unsigned long, all word sized.
type xferi struct {
	result int
	buf    uintptr
	frames uintptr
}

const xferiSize = 3 * longSize

type Config struct {
	Channels   int
	Rate       int
	Format     int
	Bits       int // physical bits per sample: 24 for S24_3LE
	PeriodSize int
	Periods    int
}

type Capture struct {
	f          *os.File
	cfg        Config
	frameBytes int
}

func ioctl(fd, req uintptr, arg unsafe.Pointer) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg)); e != 0 {
		return e
	}
	return nil
}

func ioctlArgless(fd, req uintptr) error {
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, fd, req, 0); e != 0 {
		return e
	}
	return nil
}

func Open(card, device int, cfg Config) (*Capture, error) {
	path := fmt.Sprintf("/dev/snd/pcmC%dD%dc", card, device)
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, syscall.EBUSY) || errors.Is(err, syscall.EAGAIN) {
			return nil, fmt.Errorf("%w: %s", ErrBusy, path)
		}
		return nil, err
	}
	if err := clearNonBlock(f); err != nil {
		_ = f.Close()
		return nil, err
	}

	var p hwParams
	p.init()
	p.setMask(paramAccess, accessRWInterleaved)
	p.setMask(paramFormat, cfg.Format)
	p.setMask(paramSubformat, subformatStd)
	p.setInterval(paramSampleBits, uint32(cfg.Bits))
	p.setInterval(paramFrameBits, uint32(cfg.Bits*cfg.Channels))
	p.setInterval(paramChannels, uint32(cfg.Channels))
	p.setInterval(paramRate, uint32(cfg.Rate))
	p.setInterval(paramPeriodSize, uint32(cfg.PeriodSize))
	p.setInterval(paramPeriods, uint32(cfg.Periods))

	if err := ioctl(f.Fd(), ioctlHwParams, unsafe.Pointer(&p)); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("hw_params: %w", err)
	}
	if err := granted(&p, cfg); err != nil {
		_ = f.Close()
		return nil, err
	}
	if err := ioctlArgless(f.Fd(), ioctlPrepare); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("prepare: %w", err)
	}
	if err := ioctlArgless(f.Fd(), ioctlStart); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("start: %w", err)
	}

	return &Capture{
		f:          f,
		cfg:        cfg,
		frameBytes: cfg.Channels * cfg.Bits / 8,
	}, nil
}

func (c *Capture) FrameBytes() int { return c.frameBytes }

func (c *Capture) Read(buf []byte) (int, error) {
	frames := len(buf) / c.frameBytes
	if frames == 0 {
		return 0, fmt.Errorf("buffer smaller than one frame (%d bytes)", c.frameBytes)
	}
	x := xferi{
		buf:    uintptr(unsafe.Pointer(&buf[0])),
		frames: uintptr(frames),
	}
	if err := ioctl(c.f.Fd(), ioctlReadi, unsafe.Pointer(&x)); err != nil {
		if err == syscall.EPIPE {
			_ = ioctlArgless(c.f.Fd(), ioctlPrepare)
			_ = ioctlArgless(c.f.Fd(), ioctlStart)
			return 0, ErrOverrun
		}
		return 0, err
	}
	return int(x.result) * c.frameBytes, nil
}

var ioctlDelay = ioc(2, 'A', 0x21, unsafe.Sizeof(int(0)))

func delay(fd uintptr) (int, error) {
	var d int
	if err := ioctl(fd, ioctlDelay, unsafe.Pointer(&d)); err != nil {
		return 0, err
	}
	return d, nil
}

func (c *Capture) Delay() (int, error) { return delay(c.f.Fd()) }

func (c *Capture) Close() error {
	_ = ioctlArgless(c.f.Fd(), ioctlDrop)
	return c.f.Close()
}

var ErrOverrun = fmt.Errorf("alsa: capture overrun")
