package ble

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// One reader on the line, and everything that wants the controller goes through it.
//
// There is a single UART and nothing in a packet says which reader it is for, so two goroutines
// reading it take packets out from under each other. That does not look like a conflict from
// either side: it reads as a scan that misses advertisements, or a link that drops audio, or a
// command the chip appears to have ignored. This is the one reader, and it hands each packet to
// whoever asked for that kind — which is what lets the proxy scan while a phone is connected.
//
// Bring-up is not in here. The chip downloading its firmware answers a strict sequence with the
// loader's own framing, and a demultiplexer in the middle of that is a hazard rather than a help.

// deep is how many packets may be waiting for one taker.
//
// Audio arrives faster than anything else and is the reason this is not shallow: a link handler
// held up briefly by a decode must not lose packets. A taker that stays behind this far is wedged,
// and dropping is how that becomes visible rather than the reader blocking on it.
const deep = 256

// line is the whole of what a session needs from the chip: take the next packet, put bytes on the
// wire. *Port is one, and so is a script, which is what lets the demultiplexing be tested for races
// on a machine that is not a 32 bit ARM.
type line interface {
	next(time.Duration) (packet, error)

	// Send writes without waking first. Waking means reading, and the reader is this session's
	// goroutine, so a writer that roused would be the second reader on the line.
	Send([]byte) (int, error)
	Wake() error
	Asleep() bool
}

// Session is the reader, and the way to ask the controller anything.
type Session struct {
	p line

	// wmu serialises writes. Two commands interleaved halfway through are not commands, and the
	// controller recovers from that by discarding until it finds something it recognises.
	wmu sync.Mutex

	mu sync.Mutex

	// answers is who is waiting on a command, by opcode. One at a time per opcode: two of the same
	// command in flight cannot be told apart from their answers.
	answers map[uint16]chan event

	// takers is everyone reading packets as they arrive.
	takers []*taker

	// dropped counts packets that had nowhere to go, reported now and then rather than each time.
	dropped uint64

	err  error
	done chan struct{}
}

// taker is one reader's share of the line.
type taker struct {
	wants func(packet) bool
	ch    chan packet
}

// Reader starts the one reader on a port whose bring-up has finished, for a caller holding its own
// port rather than going through the shared radio.
func Reader(ctx context.Context, p *Port) *Session {
	s := newSession(p)
	go s.serve(ctx)
	return s
}

func newSession(p line) *Session {
	return &Session{
		p:       p,
		answers: map[uint16]chan event{},
		done:    make(chan struct{}),
	}
}

// serve reads the line until ctx ends or the port fails. It is the only caller of Port.next.
func (s *Session) serve(ctx context.Context) {
	defer close(s.done)

	for ctx.Err() == nil {
		got, err := s.p.next(awake)
		switch {
		case errors.Is(err, errQuiet):
			// A quiet line is ordinary: a paired phone playing nothing says nothing.
			continue
		case err != nil:
			s.fail(fmt.Errorf("ble: reading the line: %w", err))
			return
		}

		s.route(got)
	}
	s.fail(ctx.Err())
}

// route wakes whoever asked for this packet.
//
// A completion somebody was waiting on goes to them and stops there: it answers a command this
// stack sent, which is plumbing rather than news. One nobody was waiting on carries on to the
// takers, because then it answers something a caller sent and the caller is who wants it.
func (s *Session) route(got packet) {
	if got.kind == typeEvent {
		if op, ok := addressed(got.event); ok {
			s.mu.Lock()
			waiter := s.answers[op]
			delete(s.answers, op)
			s.mu.Unlock()

			if waiter != nil {
				waiter <- got.event
				return
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, t := range s.takers {
		if !t.wants(got) {
			continue
		}
		select {
		case t.ch <- got:
		default:
			if s.dropped++; s.dropped%100 == 1 {
				slog.Warn("bluetooth packets dropped, a reader is not keeping up",
					"times", s.dropped, "waiting", deep)
			}
		}
	}
}

// addressed is the opcode a command completion or status belongs to.
func addressed(e event) (uint16, bool) {
	switch {
	case e.Code == eventCommandComplete && len(e.Params) >= 3:
		return binary.LittleEndian.Uint16(e.Params[1:]), true
	case e.Code == eventCommandStatus && len(e.Params) >= 4:
		return binary.LittleEndian.Uint16(e.Params[2:]), true
	}
	return 0, false
}

// fail records why the line stopped and lets go of everyone waiting on it.
func (s *Session) fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.err == nil {
		s.err = err
	}
	for op, waiter := range s.answers {
		close(waiter)
		delete(s.answers, op)
	}
	for _, t := range s.takers {
		close(t.ch)
	}
	s.takers = nil
}

// Err is why the line stopped, or nil while it is running.
func (s *Session) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Write puts bytes on the line, waking the chip first if it is sleeping.
func (s *Session) Write(b []byte) (int, error) {
	s.wmu.Lock()
	defer s.wmu.Unlock()

	if err := s.rouse(); err != nil {
		return 0, err
	}
	return s.p.Send(b)
}

// noticed is how often a writer looks at whether the chip has come back. The reader is already
// reading, so the acknowledgement is seen as soon as it lands and this is only the looking.
const noticed = 2 * time.Millisecond

// rouse asks the chip to wake and waits for the reader to see that it has.
//
// The waiting is a poll rather than a read on purpose. Reading here is what the old code did, and
// it made every writer a second reader on a line that only supports one — which took packets out
// from under the loop that was carrying audio.
func (s *Session) rouse() error {
	if !s.p.Asleep() {
		return nil
	}

	deadline := time.Now().Add(answers)
	var asked time.Time

	for s.p.Asleep() {
		if !time.Now().Before(deadline) {
			return fmt.Errorf("ble: the chip did not wake in %v", answers)
		}
		if time.Since(asked) >= wakeAgain {
			if err := s.p.Wake(); err != nil {
				return fmt.Errorf("ble: waking the chip: %w", err)
			}
			asked = time.Now()
		}
		time.Sleep(noticed)
	}
	return nil
}

// Ask sends a command and waits for the answer addressed to it.
//
// By opcode rather than by whatever arrives next: with a scan running there is almost always
// something else on the line, and the first packet back is usually an advertisement.
func (s *Session) Ask(cmd []byte, within time.Duration) (event, error) {
	if len(cmd) < 3 {
		return event{}, fmt.Errorf("ble: %d bytes is not a command", len(cmd))
	}
	op := binary.LittleEndian.Uint16(cmd[1:])

	back := make(chan event, 1)

	s.mu.Lock()
	if s.err != nil {
		err := s.err
		s.mu.Unlock()
		return event{}, err
	}
	if _, already := s.answers[op]; already {
		s.mu.Unlock()
		return event{}, fmt.Errorf("ble: %#04x is already in flight", op)
	}
	s.answers[op] = back
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.answers, op)
		s.mu.Unlock()
	}()

	if _, err := s.Write(cmd); err != nil {
		return event{}, fmt.Errorf("ble: writing %#04x: %w", op, err)
	}

	select {
	case e, ok := <-back:
		if !ok {
			return event{}, fmt.Errorf("ble: the line stopped while %#04x was in flight", op)
		}
		return e, nil
	case <-time.After(within):
		return event{}, fmt.Errorf("%w: %#04x was not answered in %v", errQuiet, op, within)
	}
}

// Command sends one command and gives back the parameters of its completion, having checked the
// status byte. This is what most of the stack wants: say a thing, know it was accepted.
func (s *Session) Command(opcode uint16, params ...byte) ([]byte, error) {
	cmd, err := command(opcode, params...)
	if err != nil {
		return nil, err
	}

	e, err := s.Ask(cmd, answers)
	if err != nil {
		return nil, err
	}
	return complete(e, opcode)
}

// Take registers for packets until the returned function is called. wants runs on the reader, so
// it decides rather than works.
func (s *Session) Take(wants func(packet) bool) (<-chan packet, func()) {
	t := &taker{wants: wants, ch: make(chan packet, deep)}

	s.mu.Lock()
	s.takers = append(s.takers, t)
	s.mu.Unlock()

	var once sync.Once
	return t.ch, func() {
		once.Do(func() {
			s.mu.Lock()
			defer s.mu.Unlock()

			for i, have := range s.takers {
				if have == t {
					s.takers = append(s.takers[:i], s.takers[i+1:]...)
					close(t.ch)
					return
				}
			}
		})
	}
}

// Events takes every event except the ones a scan is for, which is what a link wants.
func (s *Session) Events() (<-chan packet, func()) {
	return s.Take(func(g packet) bool {
		return g.kind != typeEvent || g.event.Code != eventLEMeta
	})
}

// Sniff reports everything that arrives for a while, for telling a chip that said nothing from one
// whose answer nobody was waiting for.
//
// A taker like any other, so it watches the line rather than taking it: whatever else is running
// keeps running and sees the same packets.
func (s *Session) Sniff(within time.Duration) []string {
	got, done := s.Take(func(packet) bool { return true })
	defer done()

	var out []string
	until := time.After(within)

	for {
		select {
		case g, ok := <-got:
			if !ok {
				return out
			}
			if g.kind == typeACL {
				out = append(out, fmt.Sprintf("data handle %#04x  % x", g.acl.Handle, g.acl.Data))
				continue
			}
			out = append(out, fmt.Sprintf("event %#02x  % x", g.event.Code, g.event.Params))
		case <-until:
			return out
		}
	}
}

// Reports takes the advertising a scan is for, and nothing else.
func (s *Session) Reports() (<-chan packet, func()) {
	return s.Take(func(g packet) bool {
		return g.kind == typeEvent && g.event.Code == eventLEMeta
	})
}
