//go:build linux

package ble

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"
)

// The rates the chip speaks. It starts at 115200 because that is what it comes out of reset at,
// and the whole point of the patch download is to get somewhere useful afterwards.
const (
	slow = unix.B115200
	fast = unix.B3000000
)

// Port is the chip's UART.
type Port struct {
	f   *os.File
	tty bool

	// baud and flow are the line as it is set, so either can be changed without the other.
	baud uint32
	flow bool

	// What has been read and not yet handed over: a read for one thing turns up another's.
	stream

	// asleep is whether the chip has said it is sleeping and has to be woken before it will take
	// anything. False until it says otherwise: the loader has no sleep protocol at all.
	//
	// Atomic because the reader sets it and a writer waiting to be let go reads it.
	asleep atomic.Bool

	// wmu makes one write one packet. The reader acknowledges a wake from its own goroutine while
	// somebody else is sending, and two writes interleaved are not packets.
	wmu sync.Mutex
}

// Open takes the tty and puts it in the shape the chip expects: eight bits, no parity, and
// hardware flow control.
//
// Flow control is not optional here and is the usual reason a bring-up gets nothing back. The chip
// asserts CTS when it is ready and the patch download pushes thirty kilobytes at it; without RTS
// and CTS the first segment that arrives while it is busy is dropped, and a dropped segment is a
// download that never completes and never says why.
func Open(path string) (*Port, error) {
	// Not the controlling terminal, and do not wait on carrier — this is a chip, not a modem.
	f, err := os.OpenFile(path, unix.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("ble: opening %s: %w", path, err)
	}

	p := &Port{f: f, tty: true}
	if err := p.configure(slow, true); err != nil {
		f.Close()
		return nil, err
	}
	return p, nil
}

// OpenNode takes an HCI character device, which speaks H4 with no line to configure.
func OpenNode(path string) (*Port, error) {
	// A blocking open of /dev/stpbt never returns: the driver powers the chip on inside it.
	f, err := os.OpenFile(path, unix.O_RDWR|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("ble: opening %s: %w", path, err)
	}
	return &Port{f: f}, nil
}

// configure sets the line, at whatever speed and with flow control on or off.
func (p *Port) configure(baud uint32, flow bool) error {
	t := unix.Termios{
		// Read, ignore the modem lines for ownership, eight bits.
		Cflag: unix.CREAD | unix.CLOCAL | unix.CS8 | baud,
		Iflag: unix.IGNPAR,
	}
	if flow {
		t.Cflag |= unix.CRTSCTS
	}

	// Raw: a read returns whatever has arrived rather than waiting for a line, and waits a tenth
	// of a second for the first byte. HCI has no line endings and a packet can be one byte.
	t.Cc[unix.VMIN] = 0
	t.Cc[unix.VTIME] = 1

	if err := unix.IoctlSetTermios(int(p.f.Fd()), unix.TCSETS, &t); err != nil {
		return fmt.Errorf("ble: setting the line: %w", err)
	}
	p.baud, p.flow = baud, flow
	return nil
}

// Speed changes the rate, after the chip has been told to change its own.
//
// The order matters and only works one way round: the chip answers the baud command at the old
// rate and then switches, so this has to happen after that answer and before anything else is
// said.
func (p *Port) Speed(baud uint32) error { return p.configure(baud, p.flow) }

// Flow turns hardware flow control on and off, which has to come off across a rate change: the
// lines mean nothing while the two ends disagree about the rate.
func (p *Port) Flow(on bool) error { return p.configure(p.baud, on) }

// Drain waits for everything written to be on the wire, which a write returning does not mean.
//
// Polled rather than tcdrain: that sleeps uninterruptibly, so a chip that stops raising CTS takes
// the thread into D state still holding the tty, and no deadline reaches it.
func (p *Port) Drain(within time.Duration) error {
	if !p.tty {
		return nil
	}
	deadline := time.Now().Add(within)

	for {
		left, err := unix.IoctlGetInt(int(p.f.Fd()), unix.TIOCOUTQ)
		if err != nil {
			return fmt.Errorf("ble: asking what is left to send: %w", err)
		}
		if left == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("ble: %d bytes still unsent after %v", left, within)
		}
		time.Sleep(time.Millisecond)
	}
}

// Flush throws away whatever is in either direction, for starting again after something went
// wrong mid-exchange.
func (p *Port) Flush() error {
	p.held = nil
	if !p.tty {
		return nil
	}
	return unix.IoctlSetInt(int(p.f.Fd()), unix.TCFLSH, unix.TCIOFLUSH)
}

func (p *Port) Close() error { return p.f.Close() }

// readSome reads what has arrived. /dev/stpbt answers an empty read with zero bytes rather than
// EAGAIN, which os.File reports as EOF.
func (p *Port) readSome(buf []byte) (int, error) {
	n, err := p.f.Read(buf)
	if !p.tty && n == 0 && errors.Is(err, io.EOF) {
		time.Sleep(time.Millisecond)
		return 0, unix.EAGAIN
	}
	return n, err
}

// Write puts a packet on the line, waking the chip first if it is sleeping.
func (p *Port) Write(b []byte) (int, error) {
	if err := p.rouse(); err != nil {
		return 0, err
	}
	return p.send(b)
}

// send bounds itself. With CRTSCTS the kernel holds a write until the chip raises CTS, and a chip
// that never does is a wait with no end: a five byte command fits the FIFO and goes out regardless,
// a 245 byte segment does not.
func (p *Port) send(b []byte) (int, error) {
	p.wmu.Lock()
	defer p.wmu.Unlock()

	if err := p.f.SetWriteDeadline(time.Now().Add(answers)); err != nil {
		return 0, fmt.Errorf("ble: %w", err)
	}
	return p.f.Write(b)
}

// Send puts bytes on the line without waking the chip first.
//
// For a caller whose reader is a goroutine of its own: rousing means reading, and a second reader
// takes packets out from under the first. Such a caller waits for Asleep to clear instead.
func (p *Port) Send(b []byte) (int, error) { return p.send(b) }

// Wake asks the chip to come back, without waiting for it to say it has.
func (p *Port) Wake() error {
	_, err := p.send([]byte{ibsWake})
	return err
}

// ibs takes a sleep-protocol byte off the front of what has been read and acts on it.
//
// They arrive between packets rather than inside one, so the front is the only place they can be.
func (p *Port) ibs() bool {
	if len(p.held) == 0 {
		return false
	}

	switch p.held[0] {
	case ibsSleep:
		p.asleep.Store(true)
	case ibsWakeAck:
		p.asleep.Store(false)
	case ibsWake:
		// The chip waking us rather than the other way round. It waits for the acknowledgement
		// before saying anything else.
		p.asleep.Store(false)
		p.send([]byte{ibsWakeAck})
	default:
		return false
	}

	p.held = p.held[1:]
	return true
}

// rouse wakes the chip, if it has said it is sleeping.
//
// The running firmware manages its own power in band: one 0xfe and it ignores everything until
// woken. Not speaking this reads as a controller that died after the reset.
func (p *Port) rouse() error {
	if !p.asleep.Load() {
		return nil
	}

	deadline := time.Now().Add(answers)
	buf := make([]byte, 64)

	// Asked again every so often rather than once. A chip in deep sleep spends the first byte
	// waking its own UART and never sees it as a wake, so a single one is a handshake that is lost
	// about as often as it lands.
	var asked time.Time

	for time.Now().Before(deadline) {
		if time.Since(asked) >= wakeAgain {
			if _, err := p.send([]byte{ibsWake}); err != nil {
				return fmt.Errorf("ble: waking the chip: %w", err)
			}
			asked = time.Now()
		}

		// Whole packets, not just the sleep bytes at the front: the acknowledgement arrives behind
		// whatever else the line is carrying. A kind nothing matches, so everything is parsed and
		// filed for whoever wanted it.
		p.drain(p.ibs)
		if !p.asleep.Load() {
			return nil
		}

		// Short of the deadline, so a chip that did not hear the first one is asked again rather
		// than waited out.
		until := time.Now().Add(wakeAgain)
		if until.After(deadline) {
			until = deadline
		}
		if err := p.f.SetReadDeadline(until); err != nil {
			return fmt.Errorf("ble: %w", err)
		}

		n, err := p.readSome(buf)
		switch {
		case n > 0:
			p.held = append(p.held, buf[:n]...)
		case errors.Is(err, os.ErrDeadlineExceeded), errors.Is(err, unix.EAGAIN), err == nil:
			// Nothing yet, which is the ordinary case between asking and being answered.
		default:
			return fmt.Errorf("ble: reading: %w", err)
		}
	}
	return fmt.Errorf("ble: the chip did not wake in %v", answers)
}

// Settle reads without wanting anything, so what the chip announces is heard rather than left for
// the next read to trip over.
func (p *Port) Settle(within time.Duration) {
	deadline := time.Now().Add(within)
	buf := make([]byte, 512)

	for time.Now().Before(deadline) {
		if err := p.f.SetReadDeadline(deadline); err != nil {
			return
		}

		n, err := p.readSome(buf)
		if n > 0 {
			p.held = append(p.held, buf[:n]...)
			for p.ibs() {
			}
			continue
		}
		if err != nil && !errors.Is(err, unix.EAGAIN) {
			return
		}
	}
	for p.ibs() {
	}
}

// Ask writes a command and reads until the chip answers or the time is up.
func (p *Port) Ask(cmd []byte, within time.Duration) (event, error) {
	if _, err := p.Write(cmd); err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			return event{}, fmt.Errorf("ble: the line would not take %d bytes in %v, so the chip is "+
				"not raising CTS", len(cmd), answers)
		}
		return event{}, fmt.Errorf("ble: writing: %w", err)
	}
	return p.Read(within)
}

// Raw reads bytes off the line and gives back whatever arrived, parsed or not, for finding out
// what the chip says rather than checking it said the expected thing.
func (p *Port) Raw(within time.Duration) ([]byte, error) {
	deadline := time.Now().Add(within)

	got := p.held
	p.held = nil

	buf := make([]byte, 512)
	for time.Now().Before(deadline) {
		if err := p.f.SetReadDeadline(deadline); err != nil {
			return got, fmt.Errorf("ble: %w", err)
		}

		n, err := p.readSome(buf)
		if n > 0 {
			got = append(got, buf[:n]...)
			continue
		}
		if errors.Is(err, os.ErrDeadlineExceeded) {
			break
		}
		if err != nil && !errors.Is(err, unix.EAGAIN) {
			return got, fmt.Errorf("ble: reading: %w", err)
		}
	}
	return got, nil
}

// Read waits for one event, keeping what arrives on the port: a UART splits packets where it
// likes, and some commands are answered twice, so bytes past this event belong to the next read.
func (p *Port) Read(within time.Duration) (event, error) { return p.read(within) }

// Asleep is whether the chip has said it is sleeping, which is what a write has to wake it out of.
//
// Worth asking separately: the flag latches on one 0xfe off the line, so a byte read at the wrong
// offset leaves it set on a chip that is wide awake, and every write after that fails to wake
// something that was never asleep.
func (p *Port) Asleep() bool { return p.asleep.Load() }

// Sniff reads the line for a while and reports every packet that arrived.
//
// For a command that timed out: the difference between a chip that said nothing and one whose
// answer nobody was waiting for.
func (p *Port) Sniff(within time.Duration) []string {
	deadline := time.Now().Add(within)
	buf := make([]byte, 512)

	for time.Now().Before(deadline) {
		if err := p.f.SetReadDeadline(deadline); err != nil {
			break
		}

		n, err := p.readSome(buf)
		if n > 0 {
			p.held = append(p.held, buf[:n]...)
		} else if err != nil && !errors.Is(err, unix.EAGAIN) && !errors.Is(err, os.ErrDeadlineExceeded) {
			break
		}

		p.drain(p.ibs)
	}

	var out []string
	for {
		e, ok := p.take()
		if !ok {
			break
		}
		out = append(out, fmt.Sprintf("event %#02x  % x", e.Code, e.Params))
	}

	if len(p.held) > 0 {
		out = append(out, fmt.Sprintf("unparsed: % x", p.held))
	}
	return out
}

func (p *Port) read(within time.Duration) (event, error) {
	var e event
	var ok bool

	if err := p.pump(within, func() bool { e, ok = p.take(); return ok }); err != nil {
		return event{}, err
	}
	return e, nil
}

// readACL is the same for data off a link.
//
// Its own queue, because a command's answer arriving between two packets of audio must not cost
// either of them: whichever the caller was not waiting for waits its turn instead of being dropped.
func (p *Port) readACL(within time.Duration) (aclPacket, error) {
	var a aclPacket
	var ok bool

	if err := p.pump(within, func() bool { a, ok = p.takeACL(); return ok }); err != nil {
		return aclPacket{}, err
	}
	return a, nil
}

// next is the oldest of either kind, for a reader handling both.
//
// Events before data when both are waiting. A command's answer decides what happens next and audio
// is already buffered, so the answer is the one worth a few microseconds.
func (p *Port) next(within time.Duration) (packet, error) {
	var got packet

	err := p.pump(within, func() bool {
		if e, ok := p.take(); ok {
			got = packet{kind: typeEvent, event: e}
			return true
		}
		if a, ok := p.takeACL(); ok {
			got = packet{kind: typeACL, acl: a}
			return true
		}
		return false
	})
	if err != nil {
		return packet{}, err
	}
	return got, nil
}

// pump reads the line until got says what it came for has arrived, or the time is up.
func (p *Port) pump(within time.Duration, got func() bool) error {
	deadline := time.Now().Add(within)
	buf := make([]byte, 512)

	for {
		// Anything already put aside for this reader, which may be the answer before the line is
		// touched at all.
		if got() {
			return nil
		}

		// Whatever has been read and not yet parsed into packets.
		if p.drain(p.ibs) {
			continue
		}

		if !time.Now().Before(deadline) {
			return fmt.Errorf("%w in %v", errQuiet, within)
		}

		// The line is opened non-blocking, so a read waits in the runtime poller rather than
		// returning. Without this it waits for as long as the chip stays quiet, which is forever.
		if err := p.f.SetReadDeadline(deadline); err != nil {
			return fmt.Errorf("ble: %w", err)
		}

		switch n, err := p.readSome(buf); {
		case n > 0:
			p.held = append(p.held, buf[:n]...)
		case errors.Is(err, os.ErrDeadlineExceeded):
			return fmt.Errorf("%w in %v", errQuiet, within)
		case errors.Is(err, unix.EAGAIN), err == nil:
			// VTIME expired with nothing on the line, which is most of the wait.
		default:
			return fmt.Errorf("ble: reading: %w", err)
		}
	}
}
