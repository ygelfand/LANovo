package alsa

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// The playback codec accepts only S16_LE.
const FormatS16_LE = 2

var (
	ioctlWritei = ioc(1, 'A', 0x50, xferiSize)
	ioctlDrain  = ioc(0, 'A', 0x44, 0)
)

type Playback struct {
	f          *os.File
	cfg        Config
	frameBytes int
	started    bool
}

// mediaserver holds this device, and a blocking open waits forever.
func OpenPlayback(card, device int, cfg Config) (*Playback, error) {
	path := fmt.Sprintf("/dev/snd/pcmC%dD%dp", card, device)
	f, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
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

	return &Playback{
		f:          f,
		cfg:        cfg,
		frameBytes: cfg.Channels * cfg.Bits / 8,
	}, nil
}

func granted(p *hwParams, cfg Config) error {
	for _, got := range []struct {
		what  string
		param int
		want  uint32
	}{
		{"period size", paramPeriodSize, uint32(cfg.PeriodSize)},
		{"periods", paramPeriods, uint32(cfg.Periods)},
		{"rate", paramRate, uint32(cfg.Rate)},
		{"channels", paramChannels, uint32(cfg.Channels)},
	} {
		if have := p.interval(got.param); have != got.want {
			return fmt.Errorf("alsa: the driver gives %s %d, not %d", got.what, have, got.want)
		}
	}
	return nil
}

func (p *Playback) FrameBytes() int { return p.frameBytes }

func (p *Playback) Write(buf []byte) (int, error) {
	frames := len(buf) / p.frameBytes
	if frames == 0 {
		return 0, fmt.Errorf("buffer smaller than one frame (%d bytes)", p.frameBytes)
	}
	x := xferi{
		buf:    uintptr(unsafe.Pointer(&buf[0])),
		frames: uintptr(frames),
	}
	if err := ioctl(p.f.Fd(), ioctlWritei, unsafe.Pointer(&x)); err != nil {
		if err == syscall.EPIPE {
			_ = ioctlArgless(p.f.Fd(), ioctlPrepare)
			p.started = false
			return 0, ErrUnderrun
		}
		return 0, err
	}

	if !p.started {
		p.started = true
		// EBADFD: the stream is already running.
		if err := ioctlArgless(p.f.Fd(), ioctlStart); err != nil && err != syscall.Errno(0x4d) {
			return int(x.result) * p.frameBytes, fmt.Errorf("start: %w", err)
		}
	}
	return int(x.result) * p.frameBytes, nil
}

func (p *Playback) Delay() (int, error) { return delay(p.f.Fd()) }

func (p *Playback) Drain() error { return ioctlArgless(p.f.Fd(), ioctlDrain) }

func (p *Playback) Close() error {
	_ = ioctlArgless(p.f.Fd(), ioctlDrop)
	return p.f.Close()
}

var ErrUnderrun = fmt.Errorf("alsa: playback underrun")

var ErrBusy = errors.New("alsa: device busy")

func clearNonBlock(f *os.File) error {
	flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, f.Fd(), syscall.F_GETFL, 0)
	if errno != 0 {
		return fmt.Errorf("alsa: reading descriptor flags: %w", errno)
	}
	if _, _, errno := syscall.Syscall(
		syscall.SYS_FCNTL,
		f.Fd(),
		syscall.F_SETFL,
		flags&^syscall.O_NONBLOCK,
	); errno != 0 {
		return fmt.Errorf("alsa: clearing O_NONBLOCK: %w", errno)
	}
	return nil
}
