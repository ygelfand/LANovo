//go:build linux

package ble

import (
	"io"
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// What a download waits for, against a line that answers nothing.
//
// The chip is told by the patch's own header whether to acknowledge segments, and download 0x03
// means it stays quiet for all of them. Waiting on the last one anyway is a two second stall and
// then a failure with the patch already on the chip, which is what it looked like on the device:
// "segment 128 of 128: ble: nothing came back in 2s".

// paired is a port whose writes go somewhere that never replies, and the other end to read them
// off. A socketpair rather than a pipe because Send writes and taken reads the same port.
func paired(t *testing.T) (*Port, *os.File) {
	t.Helper()

	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}

	// Non-blocking, or os.NewFile hands back a file the runtime cannot poll and every deadline
	// the port sets fails outright — which looks like a timeout without being one.
	for _, fd := range fds {
		if err := unix.SetNonblock(fd, true); err != nil {
			t.Fatalf("nonblock: %v", err)
		}
	}

	mine := os.NewFile(uintptr(fds[0]), "port")
	theirs := os.NewFile(uintptr(fds[1]), "chip")
	t.Cleanup(func() { mine.Close(); theirs.Close() })

	return &Port{f: mine}, theirs
}

// silent builds a patch whose header asks the chip to say nothing for the whole download.
func silent(body int, mode byte) []byte {
	raw := blob(tlvPatch, body)
	raw[4+10] = mode
	return raw
}

func TestASilentDownloadWaitsForNothing(t *testing.T) {
	p, chip := paired(t)

	// Drained, so a download bigger than the socket buffer does not wedge on its own writes.
	got := make(chan int, 1)
	go func() {
		n, _ := io.Copy(io.Discard, chip)
		got <- int(n)
	}()

	raw := silent(30764, saySilent)
	b, err := ReadBlob(raw)
	if err != nil {
		t.Fatal(err)
	}
	if b.Acked() {
		t.Fatal("download 0x03 came back as acknowledged")
	}

	// Comfortably under one segment's wait, since the point is that it never waits at all.
	done := make(chan error, 1)
	go func() { done <- Send(p, b) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a silent download that nothing answered: %v", err)
		}
	case <-time.After(answers / 2):
		t.Fatal("a silent download waited for an answer that is never coming")
	}

	// Every segment reached the line, header included. Each carries four bytes of H4 command and
	// two of TLV request on top of its slice of the file.
	p.f.Close()

	const framing = 6
	sent, want := <-got, len(raw)+framing*len(b.Segments())
	if sent != want {
		t.Errorf("%d bytes reached the chip, want %d: %d segments carrying %d",
			sent, want, len(b.Segments()), len(raw))
	}
}

func TestAnAcknowledgedDownloadStillWaits(t *testing.T) {
	p, chip := paired(t)
	go io.Copy(io.Discard, chip)

	b, err := ReadBlob(silent(3000, sayEverything))
	if err != nil {
		t.Fatal(err)
	}
	if !b.Acked() {
		t.Fatal("download 0x00 came back as silent")
	}

	// Nothing answers, so the first segment is where it gives up rather than sailing through. It
	// has to have actually waited: an error returned at once is the deadline failing, not a chip
	// that stayed quiet.
	start := time.Now()
	err = Send(p, b)
	if err == nil {
		t.Fatal("a download nothing answered came back clean")
	}
	if waited := time.Since(start); waited < answers/2 {
		t.Errorf("gave up after %v without waiting for an answer: %v", waited, err)
	}
}
