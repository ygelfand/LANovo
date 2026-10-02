//go:build linux

package ble

import (
	"testing"
	"time"
)

// Waking a chip on a busy line.
//
// The acknowledgement is one byte in a stream also carrying advertising reports, and it does not
// arrive at the front of it. Waking means reading past whole packets to reach it.

// advertising is an LE meta event, the packet the acknowledgement arrives behind.
func advertising() []byte {
	body := []byte{0x02, 0x01, 0x03, 0x00, 0x01, 0x63, 0x81, 0x46, 0x00, 0x00, 0x1e}
	return append([]byte{typeEvent, 0x3e, byte(len(body))}, body...)
}

func TestWakingPastWhatIsAlreadyOnTheLine(t *testing.T) {
	p, chip := paired(t)
	p.asleep.Store(true)

	// Reports first, acknowledgement last.
	go func() {
		chip.Write(advertising())
		chip.Write(advertising())
		chip.Write([]byte{ibsWakeAck})
	}()

	done := make(chan error, 1)
	go func() { done <- p.rouse() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("the acknowledgement was on the line and the wake still failed: %v", err)
		}
	case <-time.After(answers / 2):
		t.Fatal("waited out the deadline for a byte that had already arrived")
	}

	if p.asleep.Load() {
		t.Error("woken, but still marked asleep")
	}
}

// The reports themselves are not thrown away getting to it: they belong to whoever is scanning.
func TestWakingKeepsTheEventsItReadPastAsleep(t *testing.T) {
	p, chip := paired(t)
	p.asleep.Store(true)

	go func() {
		chip.Write(advertising())
		chip.Write([]byte{ibsWakeAck})
	}()

	if err := p.rouse(); err != nil {
		t.Fatalf("wake: %v", err)
	}

	e, err := p.Read(answers)
	if err != nil {
		t.Fatalf("the report read past while waking was lost: %v", err)
	}
	if e.Code != 0x3e {
		t.Errorf("event %#02x, want the advertising report %#02x", e.Code, 0x3e)
	}
}

// A chip that says nothing still fails, rather than the fix turning every wake into a success.
func TestWakingAChipThatNeverAnswers(t *testing.T) {
	p, _ := paired(t)
	p.asleep.Store(true)

	start := time.Now()
	if err := p.rouse(); err == nil {
		t.Fatal("a chip that never acknowledged was reported as awake")
	}
	if waited := time.Since(start); waited < answers/2 {
		t.Errorf("gave up after %v without waiting for the acknowledgement", waited)
	}
}
