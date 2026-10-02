package ble

import (
	"context"
	"encoding/binary"
	"sync"
	"testing"
	"time"
)

// The demultiplexing, hammered from several goroutines at once.
//
// The socketpair tests are the faithful ones but they only build on linux, and the device is a 32
// bit ARM where the race detector does not run. This drives the same code over a scripted line so
// that `go test -race` has something to look at — which matters here more than most places, because
// the whole point of the session is that several goroutines share one reader.

// script is a line that hands back packets from a list, forever, and swallows writes.
type script struct {
	mu   sync.Mutex
	at   int
	from []packet

	// answer, if set, is what a written command gets back, delivered before the scripted packets.
	answer  func(opcode uint16) (packet, bool)
	pending []packet
}

func (s *script) next(within time.Duration) (packet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.pending) > 0 {
		g := s.pending[0]
		s.pending = s.pending[1:]
		return g, nil
	}
	if len(s.from) == 0 {
		return packet{}, errQuiet
	}

	g := s.from[s.at%len(s.from)]
	s.at++
	return g, nil
}

func (s *script) Send(b []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.answer != nil && len(b) >= 3 && b[0] == typeCommand {
		if g, ok := s.answer(binary.LittleEndian.Uint16(b[1:])); ok {
			s.pending = append(s.pending, g)
		}
	}
	return len(b), nil
}

// The scripted chip is always awake: sleeping is the port's protocol, not the session's.
func (s *script) Wake() error  { return nil }
func (s *script) Asleep() bool { return false }

// eventPacket is one event as the reader hands it on.
func eventPacket(code byte, params ...byte) packet {
	return packet{kind: typeEvent, event: event{Code: code, Params: params}}
}

// completePacket is a command completion for an opcode, with a zero status.
func completePacket(opcode uint16) packet {
	params := []byte{0x01}
	params = binary.LittleEndian.AppendUint16(params, opcode)
	return eventPacket(eventCommandComplete, append(params, 0x00)...)
}

// Several takers, several commands and a reader, all at once. Under -race this is the test that
// says whether sharing the line is actually safe.
func TestTheReaderIsSharedSafely(t *testing.T) {
	s := newSession(&script{
		from: []packet{
			eventPacket(eventLEMeta, leAdvertisingReport, 0x00),
			eventPacket(0x05, 0x00, 0x05, 0x00, 0x13),
			{kind: typeACL, acl: aclPacket{Handle: 5, Data: []byte{1, 2, 3}}},
		},
		answer: func(op uint16) (packet, bool) { return completePacket(op), true },
	})

	ctx, cancel := context.WithCancel(context.Background())
	go s.serve(ctx)

	var wg sync.WaitGroup

	// Takers coming and going while the line runs, which is what a proxy toggling does.
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				got, done := s.Reports()
				select {
				case <-got:
				case <-time.After(20 * time.Millisecond):
				}
				done()
			}
		}()
	}

	// And commands in flight throughout, each with its own opcode so they do not collide.
	for i := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			op := uint16(0x1000 + i)
			for range 20 {
				s.Command(op)
			}
		}()
	}

	// One long lived taker, the way a link is.
	wg.Add(1)
	go func() {
		defer wg.Done()

		events, release := s.Events()
		defer release()

		for range 50 {
			select {
			case <-events:
			case <-time.After(20 * time.Millisecond):
			}
		}
	}()

	wg.Wait()
	cancel()

	// The reader stops, and stopping releases everyone rather than leaving them waiting.
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		t.Fatal("the reader did not stop")
	}
	if s.Err() == nil {
		t.Error("a stopped reader has no reason recorded")
	}
}

// A taker that stops reading must not wedge the line for everyone else.
func TestASlowTakerIsDroppedNotWaitedOn(t *testing.T) {
	s := newSession(&script{from: []packet{eventPacket(eventLEMeta, leAdvertisingReport, 0x00)}})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.serve(ctx)

	// Registered and then never read from.
	_, stuck := s.Reports()
	defer stuck()

	// Somebody else has to keep getting packets regardless.
	events, release := s.Events()
	defer release()

	// The scripted line is only advertisements, so the link taker gets nothing — what matters is
	// that the reader is still running rather than blocked on the taker that stopped.
	select {
	case <-events:
	case <-time.After(300 * time.Millisecond):
	}

	if err := s.Err(); err != nil {
		t.Fatalf("the reader stopped: %v", err)
	}

	// Now prove it is still reading: a command still finds its answer.
	s.p.(*script).mu.Lock()
	s.p.(*script).answer = func(op uint16) (packet, bool) { return completePacket(op), true }
	s.p.(*script).mu.Unlock()

	if _, err := s.Command(hciReadAddress); err != nil {
		t.Fatalf("the line is wedged behind a taker that stopped reading: %v", err)
	}
}
