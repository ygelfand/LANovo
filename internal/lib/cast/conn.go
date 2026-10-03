package cast

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

// Connections, and getting a message to the right one.
//
// Each sender opens its own TCP connection and is known on it by an endpoint id. A message the
// receiver produces carries the id it is for and not the socket, so something has to hold the two
// together — otherwise a status meant for the phone in the kitchen goes down the connection to the
// one in the hall, where it is a message about a session that endpoint has never heard of.
//
// WHY WRITES ARE QUEUED
//
// A change one sender makes is told to all the others, and the end of a track is told to everyone
// by whatever noticed — which on the device is the audio path. Writing to each connection in turn
// from there means one phone that has stopped reading holds up the speaker.
//
// So each connection has a queue and a goroutine that drains it. A queue that fills is a sender
// that is not keeping up, and it is dropped rather than allowed to hold up anything else. Ordering
// within a connection is kept, which matters: a status that arrives after the one that superseded
// it leaves a sender showing the wrong thing.

// readChunk is how much is taken off the socket at once. A media status with artwork urls is a few
// kilobytes; anything larger arrives over several reads and is joined up.
const readChunk = 4096

// outbound is how many messages may be waiting for one sender.
//
// Generous for what this sends — a handful per change — and small enough that a sender which has
// stopped reading is noticed rather than accumulating.
const outbound = 32

// ErrGone is a connection that has been closed, from either end.
var ErrGone = errors.New("cast: the connection has gone")

// Conn is one sender's connection, framed.
type Conn struct {
	conn net.Conn

	held []byte
	buf  []byte

	out    chan Message
	closed chan struct{}
	once   sync.Once

	// Trace sees every message read and every message written, before either is acted on.
	Trace func(in bool, m Message)
}

// Peer is the far end's address.
func (c *Conn) Peer() string { return c.conn.RemoteAddr().String() }

// NewConn wraps a connection and starts draining its queue.
func NewConn(c net.Conn) *Conn {
	k := &Conn{
		conn:   c,
		out:    make(chan Message, outbound),
		closed: make(chan struct{}),
	}

	go k.writing()
	return k
}

// writing drains the queue until the connection goes.
func (c *Conn) writing() {
	for {
		select {
		case m := <-c.out:
			if _, err := c.conn.Write(Frame(m)); err != nil {
				c.Close()
				return
			}
		case <-c.closed:
			return
		}
	}
}

// Read waits for one whole message.
//
// A short buffer means more is coming and is not reported; anything else means the stream cannot be
// resynchronised, because a length that was wrong once puts every byte after it in the wrong place.
// That is a connection to close rather than an error to carry on from.
func (c *Conn) Read() (Message, error) {
	for {
		m, took, err := Next(c.held)
		if err == nil {
			c.held = c.held[took:]
			if c.Trace != nil {
				c.Trace(true, m)
			}
			return m, nil
		}
		if !errors.Is(err, ErrShort) {
			return Message{}, err
		}

		if c.buf == nil {
			c.buf = make([]byte, readChunk)
		}

		n, err := c.conn.Read(c.buf)
		if n > 0 {
			c.held = append(c.held, c.buf[:n]...)
			continue
		}
		if err != nil {
			return Message{}, err
		}
	}
}

// Write queues a message.
//
// It does not wait for the socket. A full queue is a sender that has stopped reading, and it is
// closed rather than allowed to hold up whatever was telling it something — which is usually the
// thing playing the audio.
func (c *Conn) Write(m Message) error {
	select {
	case <-c.closed:
		return ErrGone
	default:
	}

	if c.Trace != nil {
		c.Trace(false, m)
	}
	select {
	case c.out <- m:
		return nil
	case <-c.closed:
		return ErrGone
	default:
		c.Close()
		return fmt.Errorf("cast: %s is not reading, so it has been dropped", m.Destination)
	}
}

// Close ends it, from this side.
//
// Closing the socket as well as the queue is what stops the reader waiting: a connection whose
// writer gave up but whose reader is still blocked is a goroutine that never returns.
func (c *Conn) Close() error {
	c.once.Do(func() {
		close(c.closed)
		c.conn.Close()
	})
	return nil
}

// Service is the receiver with its connections, and the routing between them.
//
// Safe for concurrent use, which the receiver underneath is not: every connection's reader takes
// the lock before handing a message over, so the state machine still only ever runs one at a time.
type Service struct {
	// Receiver is the state machine. Configure it — the player, the callbacks — before serving.
	Receiver *Receiver

	// Fault is called for a message that could not be understood. The connection is kept: a
	// sender's bad payload is its bug, and dropping the connection over it turns that into a
	// device that cannot be cast to. Nil discards them.
	Fault func(error)

	mu    sync.Mutex
	conns map[string]*Conn
}

// NewService is a service around a receiver.
func NewService(r *Receiver) *Service {
	return &Service{Receiver: r, conns: map[string]*Conn{}}
}

// Serve reads one connection until it ends, answering as it goes.
//
// The sender's endpoint id is learned from the first message it sends, because a connection does
// not carry one until then. Everything before that can still be answered — the reply goes back the
// way it came — but nothing can be broadcast to it.
func (s *Service) Serve(c *Conn) error {
	var known string

	defer func() {
		c.Close()
		if known != "" {
			s.forget(known)
		}
	}()

	for {
		m, err := c.Read()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		if m.Source != "" && m.Source != known {
			known = m.Source
			s.remember(known, c)
		}

		out, fault := s.handle(m)
		if fault != nil && s.Fault != nil {
			s.Fault(fault)
		}

		for _, a := range out {
			// Back the way it came when it is for this sender, and to whichever connection holds
			// the endpoint otherwise.
			if a.Destination == m.Source {
				if err := c.Write(a); err != nil {
					return err
				}
				continue
			}
			s.Send(a)
		}
	}
}

// handle runs one message through the receiver under the lock.
func (s *Service) handle(m Message) ([]Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Receiver.Receive(m)
}

// Send delivers a message to whichever connection holds its destination.
//
// A destination nobody is holding is dropped rather than reported: a sender that has gone away
// while a broadcast was being built is ordinary, not a fault.
func (s *Service) Send(m Message) {
	s.mu.Lock()
	var to []*Conn
	if m.Destination == Broadcast {
		seen := map[*Conn]bool{}
		for _, c := range s.conns {
			if !seen[c] {
				seen[c] = true
				to = append(to, c)
			}
		}
	} else if c := s.conns[m.Destination]; c != nil {
		to = append(to, c)
	}
	s.mu.Unlock()

	for _, c := range to {
		if err := c.Write(m); err != nil && !errors.Is(err, ErrGone) && s.Fault != nil {
			s.Fault(err)
		}
	}
}

// Announce sends messages the receiver produced on its own, with nobody having asked — a track
// finishing, or something at this end changing what is playing.
func (s *Service) Announce(out []Message) {
	for _, m := range out {
		s.Send(m)
	}
}

// Report is the device's volume, told to everyone watching when it moved.
func (s *Service) Report(level float64, muted bool) {
	s.mu.Lock()
	out := s.Receiver.Report(level, muted)
	s.mu.Unlock()
	s.Announce(out)
}

// Publish is what an application is playing on its own, told to everyone watching.
func (s *Service) Publish(control Playing, p *Published) {
	s.mu.Lock()
	out := s.Receiver.Publish(control, p)
	s.mu.Unlock()

	s.Announce(out)
}

// End stops the running application from this end, told to everyone watching.
func (s *Service) End() {
	s.mu.Lock()
	out := s.Receiver.End()
	s.mu.Unlock()

	s.Announce(out)
}

// Finished is the player having reached the end, told to everyone watching.
func (s *Service) Finished() {
	s.mu.Lock()
	out := s.Receiver.Finished()
	s.mu.Unlock()

	s.Announce(out)
}

func (s *Service) remember(id string, c *Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.conns[id] = c
}

// forget drops a connection, and tells the receiver its sender has gone.
//
// Without this a sender that pulled the plug stays in the receiver's list for ever, and every
// change afterwards builds a broadcast to somebody who is not there.
func (s *Service) forget(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.conns, id)
	delete(s.Receiver.senders, id)
}

// Senders is how many connections are held, for a caller deciding whether anything is listening.
func (s *Service) Senders() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.conns)
}
