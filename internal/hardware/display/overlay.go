package display

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/ygelfand/LANovo/internal/layout"
)

type overlay interface {
	show() error
	close()
}

func openOverlay(w, h int) (overlay, *Panel, error) {
	if _, err := os.Stat(layout.DispMgrDevice); err == nil {
		return openSession(w, h)
	}
	return openCommit(w, h)
}

func overlayPanel(buf *ionBuffer, w, h int) *Panel {
	return &Panel{mem: buf.mem, stride: w * 4, fbW: w, fbH: h, rot: Mounted(), Width: w, Height: h}
}

const (
	mdssAtomicCommit = 3<<30 | 4<<16 | 'S'<<8 | 128
	mdssCommitV1     = 1 << 16
	mdssValidate     = 0x01
	mdssPipeRGB0     = 8
	mdssRGBA8888     = 13
	mdssBlendOpaque  = 1
	mdssSystemHeap   = 1 << 25

	mdssInputBytes  = 180
	mdssCommitBytes = 84
)

type commitOverlay struct {
	fb   *os.File
	buf  *ionBuffer
	w, h int
}

func openCommit(w, h int) (overlay, *Panel, error) {
	fb, err := os.OpenFile(layout.FBDevice, os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	buf, err := ionAllocate(w*h*4, mdssSystemHeap)
	if err != nil {
		fb.Close()
		return nil, nil, err
	}
	o := &commitOverlay{fb: fb, buf: buf, w: w, h: h}
	if err := o.commit(mdssValidate); err != nil {
		o.close()
		return nil, nil, err
	}
	return o, overlayPanel(buf, w, h), nil
}

func (o *commitOverlay) show() error { return o.commit(0) }

func (o *commitOverlay) commit(flags uint32) error {
	in := make([]byte, mdssInputBytes)
	put := func(at int, v uint32) { binary.LittleEndian.PutUint32(in[at:], v) }
	put(4, mdssPipeRGB0)
	in[10] = 0xff
	put(24, mdssBlendOpaque)
	for at, v := range []int{0, 0, o.w, o.h, 0, 0, o.w, o.h} {
		put(32+4*at, uint32(v))
	}
	put(68, uint32(o.w))
	put(72, uint32(o.h))
	put(76, mdssRGBA8888)
	for i := range 4 {
		put(80+12*i, ^uint32(0))
	}
	put(80, uint32(o.buf.fd))
	put(88, uint32(o.w*4))
	put(128, 1)
	put(140, ^uint32(0))

	req := make([]byte, mdssCommitBytes)
	binary.LittleEndian.PutUint32(req[0:], mdssCommitV1)
	binary.LittleEndian.PutUint32(req[4:], flags)
	binary.LittleEndian.PutUint32(req[8:], ^uint32(0))
	binary.LittleEndian.PutUint32(req[44:], uint32(uintptr(unsafe.Pointer(&in[0]))))
	binary.LittleEndian.PutUint32(req[48:], 1)
	binary.LittleEndian.PutUint32(req[56:], ^uint32(0))
	err := rawIoctl(o.fb.Fd(), mdssAtomicCommit, req)
	runtime.KeepAlive(in)
	for _, at := range []int{8, 56} {
		if f := int32(binary.LittleEndian.Uint32(req[at:])); f >= 0 && err == nil {
			syscall.Close(int(f))
		}
	}
	if err != nil {
		return fmt.Errorf("display: overlay commit: %w (layer error %d)", err, int32(binary.LittleEndian.Uint32(in[152:])))
	}
	return nil
}

func (o *commitOverlay) close() {
	o.fb.Close()
	o.buf.close()
}

const (
	dispPrepareInput   = 0x40244fcc
	dispSetInputBuffer = 0x45044fce
	dispTriggerSession = 0x40244fcb
	dispGetSessionInfo = 0x40544fd0
	dispPrimary        = 1 << 16
	dispRGBA8888       = 6<<8 | 4
	dispDirectLink     = 1
	dispMultimediaHeap = 1 << 10
)

type sessionOverlay struct {
	dev   *os.File
	buf   *ionBuffer
	w, h  int
	layer uint32
}

func openSession(w, h int) (overlay, *Panel, error) {
	dev, err := os.OpenFile(layout.DispMgrDevice, os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	info := make([]byte, 84)
	binary.LittleEndian.PutUint32(info[0:], dispPrimary)
	if err := rawIoctl(dev.Fd(), dispGetSessionInfo, info); err != nil {
		dev.Close()
		return nil, nil, fmt.Errorf("display: session info: %w", err)
	}
	layers := binary.LittleEndian.Uint32(info[4:])
	if layers == 0 {
		dev.Close()
		return nil, nil, errors.New("display: the primary session has no layers")
	}
	buf, err := ionAllocate(w*h*4, dispMultimediaHeap)
	if err != nil {
		dev.Close()
		return nil, nil, err
	}
	o := &sessionOverlay{dev: dev, buf: buf, w: w, h: h, layer: layers - 1}
	return o, overlayPanel(buf, w, h), nil
}

func (o *sessionOverlay) show() error {
	prep := make([]byte, 36)
	put := func(b []byte, at int, v uint32) { binary.LittleEndian.PutUint32(b[at:], v) }
	put(prep, 0, dispPrimary)
	put(prep, 4, o.layer)
	put(prep, 8, 1)
	put(prep, 12, uint32(o.buf.fd))
	put(prep, 16, 1)
	if err := rawIoctl(o.dev.Fd(), dispPrepareInput, prep); err != nil {
		return fmt.Errorf("display: overlay prepare: %w", err)
	}
	if f := int32(binary.LittleEndian.Uint32(prep[24:])); f >= 0 {
		syscall.Close(int(f))
	}
	return o.set(true, binary.LittleEndian.Uint32(prep[20:]))
}

func (o *sessionOverlay) set(on bool, index uint32) error {
	in := make([]byte, 1284)
	put32 := func(at int, v uint32) { binary.LittleEndian.PutUint32(in[at:], v) }
	put16 := func(at int, v int) { binary.LittleEndian.PutUint16(in[at:], uint16(v)) }
	put32(4, dispPrimary)
	put32(8, 1)
	const c = 12
	put32(c+16, dispRGBA8888)
	put32(c+44, index)
	put32(c+48, ^uint32(0))
	put16(c+70, o.w)
	put16(c+76, o.w)
	put16(c+78, o.h)
	put16(c+84, o.w)
	put16(c+86, o.h)
	in[c+89] = 0xff
	in[c+92] = byte(o.layer)
	if on {
		in[c+93] = 1
	}
	if err := rawIoctl(o.dev.Fd(), dispSetInputBuffer, in); err != nil {
		return fmt.Errorf("display: overlay input: %w", err)
	}

	trig := make([]byte, 36)
	binary.LittleEndian.PutUint32(trig[0:], 1)
	binary.LittleEndian.PutUint32(trig[8:], dispDirectLink)
	binary.LittleEndian.PutUint32(trig[12:], dispPrimary)
	binary.LittleEndian.PutUint32(trig[20:], ^uint32(0))
	if err := rawIoctl(o.dev.Fd(), dispTriggerSession, trig); err != nil {
		return fmt.Errorf("display: overlay trigger: %w", err)
	}
	return nil
}

func (o *sessionOverlay) close() {
	o.set(false, 0)
	o.dev.Close()
	o.buf.close()
}
