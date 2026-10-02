//go:build linux

package ble

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"testing"
	"time"
)

// The one reader, driven over a socketpair with no controller behind it.
//
// What this has to get right is that two readers no longer take packets out from under each other.
// Before it, a scan and a link could not both run: the proxy had to be turned off to use the
// speaker, because whichever loop read the line first swallowed the other's packets. So the tests
// here are about coexistence, and about a command still finding its answer in a line full of
// somebody else's traffic.

// advert is an LE Meta advertising report, which is what a scan is for.
func advert(address [6]byte, rssi int8) []byte {
	body := meta(report(0x00, 0x01, address, []byte{0x02, 0x01, 0x06}, rssi))

	out := []byte{typeEvent, eventLEMeta, byte(len(body))}
	return append(out, body...)
}

// classic is an event a link cares about and a scan does not.
func classic(code byte, params ...byte) []byte {
	out := []byte{typeEvent, code, byte(len(params))}
	return append(out, params...)
}

// answerTo is the controller answering a command, with no parameters past the status.
func answerTo(opcode uint16, status byte) []byte {
	out := []byte{typeEvent, eventCommandComplete, 0x04, 0x01}
	out = binary.LittleEndian.AppendUint16(out, opcode)
	return append(out, status)
}

// reading starts a session over a socketpair and gives back the chip's end.
func reading(t *testing.T) (*Session, *os.File) {
	t.Helper()

	p, chip := paired(t)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	return Reader(ctx, p), chip
}

// The whole point: advertisements reach a scan while classic events reach a link, at the same time,
// from one line.
func TestAScanAndALinkBothGetTheirOwn(t *testing.T) {
	s, chip := reading(t)

	reports, stopScan := s.Reports()
	defer stopScan()

	events, stopLink := s.Events()
	defer stopLink()

	want := [6]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}

	// Interleaved the way they actually arrive, rather than one kind and then the other.
	for _, b := range [][]byte{
		advert(want, -64),
		classic(0x05, 0x00, 0x05, 0x00, 0x13),
		advert(want, -70),
		aclOf(0x0005, boundaryStart, []byte{1, 2, 3, 4}),
	} {
		if _, err := chip.Write(b); err != nil {
			t.Fatal(err)
		}
	}

	// The scan gets two advertisements and nothing else.
	for i := range 2 {
		g := within(t, reports)
		if g.kind != typeEvent || g.event.Code != eventLEMeta {
			t.Fatalf("advertisement %d came through as %#02x", i, g.event.Code)
		}
	}

	// The link gets the disconnection and the data, and neither advertisement.
	first := within(t, events)
	if first.kind != typeEvent || first.event.Code != 0x05 {
		t.Fatalf("the link was handed %#02x first, want the disconnection", first.event.Code)
	}

	second := within(t, events)
	if second.kind != typeACL {
		t.Fatalf("the link was handed an event second, want the data")
	}
	if !bytes.Equal(second.acl.Data, []byte{1, 2, 3, 4}) {
		t.Fatalf("the data came through as % x", second.acl.Data)
	}

	nothingMore(t, reports)
	nothingMore(t, events)
}

// A command's answer has to be found among whatever else is arriving. Taking whichever packet came
// back next is what broke when a scan was running, because the next packet is almost always an
// advertisement.
func TestAnAnswerIsFoundAmongTheAdvertisements(t *testing.T) {
	s, chip := reading(t)

	// A scan running throughout, so the line is never quiet.
	reports, stop := s.Reports()
	defer stop()

	noise := make(chan struct{})
	go func() {
		defer close(noise)
		for range 20 {
			chip.Write(advert([6]byte{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}, -50))
			time.Sleep(time.Millisecond)
		}
	}()

	// Answer the command when it turns up, the way a controller does.
	go func() {
		want, _ := command(hciReadAddress)
		got := make([]byte, len(want))

		chip.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, err := io.ReadFull(chip, got); err != nil {
			return
		}
		chip.SetReadDeadline(time.Time{})

		// Six bytes of address after the status, least significant first.
		out := []byte{typeEvent, eventCommandComplete, 0x0a, 0x01}
		out = binary.LittleEndian.AppendUint16(out, hciReadAddress)
		out = append(out, 0x00, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01)
		chip.Write(out)
	}()

	said, err := s.Command(hciReadAddress, nil...)
	if err != nil {
		t.Fatalf("asking for the address with a scan running: %v", err)
	}
	if want := []byte{0x06, 0x05, 0x04, 0x03, 0x02, 0x01}; !bytes.Equal(said, want) {
		t.Fatalf("the address came back as % x, want % x", said, want)
	}

	<-noise

	// And the scan kept its advertisements through all of it.
	if len(reports) == 0 {
		t.Error("the scan saw nothing while the command was in flight")
	}
}

// A command the controller refuses is an answer, not a silence.
func TestARefusalIsAnAnswer(t *testing.T) {
	s, chip := reading(t)

	go func() {
		want, _ := command(hciWriteSimplePairing, 0x01)
		got := make([]byte, len(want))

		chip.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, err := io.ReadFull(chip, got); err != nil {
			return
		}
		chip.Write(answerTo(hciWriteSimplePairing, 0x12))
	}()

	if _, err := s.Command(hciWriteSimplePairing, 0x01); err == nil {
		t.Fatal("a refused command came back without an error")
	}
}

// Two of the same command cannot be told apart from their answers, so the second is refused rather
// than handed the first one's.
func TestTheSameCommandTwiceAtOnceIsRefused(t *testing.T) {
	s, _ := reading(t)

	cmd, err := command(hciReadAddress)
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	go func() {
		close(started)
		s.Ask(cmd, 500*time.Millisecond)
	}()
	<-started

	// Give the first one time to register before the second tries.
	time.Sleep(50 * time.Millisecond)

	if _, err := s.Ask(cmd, 100*time.Millisecond); err == nil {
		t.Fatal("the same command twice at once was accepted")
	}
}

// Letting go means letting go: a taker that has stopped must not keep receiving, or a scan that
// ended holds the line's packets forever.
func TestLettingGoStopsTheDelivery(t *testing.T) {
	s, chip := reading(t)

	reports, stop := s.Reports()
	stop()

	if _, ok := <-reports; ok {
		t.Fatal("a released taker is still open")
	}

	// And the line carries on for everyone else.
	events, release := s.Events()
	defer release()

	if _, err := chip.Write(classic(0x05, 0x00, 0x05, 0x00, 0x13)); err != nil {
		t.Fatal(err)
	}
	if g := within(t, events); g.event.Code != 0x05 {
		t.Fatalf("the link was handed %#02x after a scan let go", g.event.Code)
	}
}

// within takes one packet, or says the reader never delivered it.
func within(t *testing.T, from <-chan packet) packet {
	t.Helper()

	select {
	case g, ok := <-from:
		if !ok {
			t.Fatal("the reader stopped")
		}
		return g
	case <-time.After(2 * time.Second):
		t.Fatal("nothing was delivered in 2s")
	}
	return packet{}
}

// nothingMore says the taker is done: no other packet should reach it.
func nothingMore(t *testing.T, from <-chan packet) {
	t.Helper()

	select {
	case g := <-from:
		t.Fatalf("something else arrived: kind %#02x code %#02x", g.kind, g.event.Code)
	case <-time.After(100 * time.Millisecond):
	}
}
