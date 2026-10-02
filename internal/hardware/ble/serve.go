package ble

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/bt/pair"
)

// Serving a classic connection: the loop that owns the line while a phone is connected.
//
// This package knows about HCI and ACL and nothing above them. What an L2CAP payload means, which
// channels exist, how a pairing is decided — none of that is here. The caller answers those, and
// this moves the bytes: reassembling fragments, honouring the controller's buffer count, and
// handing each thing to whoever decides.
//
// Kept apart that way because the two halves fail differently. A mistake here drops audio or wedges
// the link; a mistake above it answers a phone wrongly. Debugging them together is worse than
// either.

// Classic is what decides, for a link this package is only carrying.
type Classic interface {
	// Event answers an HCI event with whatever commands should go out. Returning none is ordinary:
	// most events are news rather than questions.
	Event(pair.Event) ([]pair.Command, error)

	// Data answers one L2CAP PDU with whatever should go back on the same link.
	Data(handle uint16, pdu []byte) ([][]byte, error)

	// Gone says a link has ended, so anything keyed on the handle can be let go.
	Gone(handle uint16)
}

// Sends is a Classic that also speaks without being asked, for something that changed at this end
// rather than something the phone said.
//
// Optional: a handler that only ever answers does not implement it. Everything in Classic is a
// reply, and a sink whose own volume moves has nothing to reply to.
type Sends interface {
	// Sending hands over a way to put a PDU on a link, and nil once the link has gone. What it is
	// given never blocks — it queues, and reports a queue that is full rather than waiting on it.
	Sending(send func(handle uint16, pdu []byte) error)
}

// awake is how long a read waits before looking at whether the loop should still be running. A
// quiet line is ordinary: a paired phone playing nothing says nothing.
const awake = time.Second

// queued is how many things may be waiting to go out.
//
// A sink sends little: channel and stream negotiation, and answers to remote control. The audio all
// comes the other way. Deep enough that a burst of negotiation does not touch the limit, shallow
// enough that a link which has stopped taking anything is noticed rather than buffered.
const queued = 64

// outbound is one thing to put on the line. Everything leaves through one goroutine, so the two
// kinds travel together.
type outbound struct {
	// raw is a framed HCI command, ready to write.
	raw []byte

	// handle and pdu are an L2CAP payload to fragment onto a link. raw is nil when these are set.
	handle Handle
	pdu    []byte
}

// Serve carries a classic link until ctx ends.
//
// Reading and writing are separate goroutines, and that is load bearing rather than tidy. The
// controller holds a fixed number of packets and hands them back in an event; a loop that blocked
// waiting for room would be waiting for a message only it can read.
func Serve(ctx context.Context, s *Session, on Classic) error {
	// Before anything is asked of the controller, so its answers are not the packets that arrive
	// while nobody is registered.
	got, release := s.Events()
	defer release()

	size, total, err := buffers(s)
	if err != nil {
		return fmt.Errorf("ble: asking what the controller holds: %w", err)
	}

	room, err := newCredit(size, total)
	if err != nil {
		return err
	}
	slog.Info("bluetooth link", "packet", size, "buffers", total)

	ctx, stop := context.WithCancel(ctx)
	defer stop()

	out := make(chan outbound, queued)
	go writing(ctx, s, room, size, out)

	// Something that speaks for itself gets a way to. Taken away again on the way out, so a handler
	// that outlives one link does not write into a queue nothing is draining.
	if speaks, ok := on.(Sends); ok {
		speaks.Sending(func(handle uint16, pdu []byte) error {
			return post(ctx, out, outbound{handle: Handle(handle), pdu: pdu})
		})
		defer speaks.Sending(nil)
	}

	parts := newReassembler()

	for {
		var g packet
		select {
		case <-ctx.Done():
			return nil
		case have, ok := <-got:
			if !ok {
				return fmt.Errorf("ble: serving: %w", s.Err())
			}
			g = have
		}

		if g.kind == typeACL {
			if err := carry(ctx, on, parts, out, g.acl); err != nil {
				// A malformed packet is a glitch, not a broken link. Dropping the connection over
				// one is how a stutter becomes a phone that stops playing.
				slog.Warn("bluetooth data", "err", err)
			}
			continue
		}

		if err := answer(ctx, on, parts, room, out, g.event); err != nil {
			slog.Warn("bluetooth event", "code", g.event.Code, "err", err)
		}
	}
}

// writing is the only thing that puts bytes on the line.
//
// One writer because two would interleave halfway through a packet, and because waiting for the
// controller to free a buffer has to happen somewhere that is not the reader.
func writing(ctx context.Context, s *Session, room *credit, size int, out <-chan outbound) {
	for {
		var o outbound

		select {
		case <-ctx.Done():
			return
		case o = <-out:
		}

		if o.raw != nil {
			if _, err := s.Write(o.raw); err != nil {
				slog.Warn("bluetooth writing a command", "err", err)
			}
			continue
		}

		frames, err := aclFrames(o.handle, o.pdu, size)
		if err != nil {
			slog.Warn("bluetooth framing data", "err", err)
			continue
		}

		for _, f := range frames {
			// Waiting is the point. Writing past what the controller holds is what makes one stop
			// answering rather than refuse.
			if err := room.take(ctx); err != nil {
				return
			}
			if _, err := s.Write(f); err != nil {
				slog.Warn("bluetooth writing data", "err", err)
				break
			}
		}
	}
}

// post puts something on the queue, or says the link is not keeping up.
//
// Never blocks. The reader calls this, and a reader that waits to send is a reader that is not
// reading the event which would let it send.
func post(ctx context.Context, out chan<- outbound, o outbound) error {
	select {
	case out <- o:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("ble: %d things already waiting to go out", queued)
	}
}

// answer hands an event to whoever decides and queues what they said.
func answer(ctx context.Context, on Classic, parts *reassembler, room *credit,
	out chan<- outbound, e event) error {
	// Buffers coming back is this package's own business: the caller does not know the controller
	// is counting.
	if e.Code == eventCompletedPackets {
		done, err := parseCompletedPackets(e)
		if err != nil {
			return err
		}
		for _, d := range done {
			if err := room.give(d.Count); err != nil {
				return err
			}
		}
		return nil
	}

	// A link ending is both: the caller is told, and what this package was keeping for it goes.
	if e.Code == pair.EventDisconnectionComplete {
		if h, ok := pair.Handle(e.Params); ok {
			parts.forget(Handle(h))
			on.Gone(h)
		}
	}

	said, err := on.Event(pair.Event{Code: e.Code, Params: e.Params})
	if err != nil {
		return err
	}

	for _, c := range said {
		cmd, err := command(c.Opcode, c.Params...)
		if err != nil {
			return err
		}
		if err := post(ctx, out, outbound{raw: cmd}); err != nil {
			return fmt.Errorf("sending %s: %w", c, err)
		}
	}
	return nil
}

// carry reassembles a fragment and, once a whole PDU is there, hands it over and queues the reply.
func carry(ctx context.Context, on Classic, parts *reassembler, out chan<- outbound, a aclPacket) error {
	pdu, err := parts.add(a)
	if err != nil {
		return err
	}
	if pdu == nil {
		// Part of one. The rest is still coming.
		return nil
	}

	said, err := on.Data(uint16(a.Handle), pdu)
	if err != nil {
		return err
	}

	for _, reply := range said {
		if err := post(ctx, out, outbound{handle: a.Handle, pdu: reply}); err != nil {
			return err
		}
	}
	return nil
}

// buffers asks the controller how much it will take at once.
func buffers(s *Session) (size, total int, err error) {
	said, err := s.Command(hciReadBufferSize)
	if err != nil {
		return 0, 0, err
	}
	return parseBufferSize(said)
}
