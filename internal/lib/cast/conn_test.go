package cast

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// The connection layer, over net.Pipe: a real full duplex connection with no socket and no network,
// so the routing that decides which sender hears what can be driven here rather than on a device
// with two phones in the room.

const patience = 2 * time.Second

// sender is one end of a pipe, framed, with the deadlines a test needs.
type sender struct {
	t    *testing.T
	conn net.Conn
	held []byte
}

func (s *sender) send(m Message) {
	s.t.Helper()

	s.conn.SetWriteDeadline(time.Now().Add(patience))
	if _, err := s.conn.Write(Frame(m)); err != nil {
		s.t.Fatalf("writing: %v", err)
	}
}

// next reads one message, waiting for it.
func (s *sender) next() (Message, error) {
	buf := make([]byte, readChunk)

	for {
		m, took, err := Next(s.held)
		if err == nil {
			s.held = s.held[took:]
			return m, nil
		}
		if !errors.Is(err, ErrShort) {
			return Message{}, err
		}

		s.conn.SetReadDeadline(time.Now().Add(patience))
		n, err := s.conn.Read(buf)
		if n > 0 {
			s.held = append(s.held, buf[:n]...)
			continue
		}
		if err != nil {
			return Message{}, err
		}
	}
}

// expect reads one message and fails if it does not arrive.
func (s *sender) expect() Message {
	s.t.Helper()

	m, err := s.next()
	if err != nil {
		s.t.Fatalf("reading: %v", err)
	}
	return m
}

// dial attaches a sender to a service and starts serving it.
func dial(t *testing.T, s *Service) *sender {
	t.Helper()

	ours, theirs := net.Pipe()
	t.Cleanup(func() { ours.Close(); theirs.Close() })

	go s.Serve(NewConn(theirs))

	return &sender{t: t, conn: ours}
}

// A whole conversation over a connection rather than through Receive directly.
func TestAConversationOverAConnection(t *testing.T) {
	s := NewService(NewReceiver("Kitchen"))
	s.Receiver.Player = &player{}

	phone := dial(t, s)

	phone.send(from(SenderID, NSConnection, Connect()))
	phone.send(from(SenderID, NSHeartbeat, Ping()))

	pong := phone.expect()
	if h, _ := Kind(pong.Payload); h.Type != TypePong {
		t.Fatalf("came back %q", h.Type)
	}
	if pong.Destination != SenderID {
		t.Errorf("the pong went to %q", pong.Destination)
	}

	phone.send(from(SenderID, NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))

	launched := phone.expect()
	if h, _ := Kind(launched.Payload); h.Type != TypeStatus {
		t.Fatalf("a launch came back as %q", h.Type)
	}
}

// Two senders on two connections. A status meant for one must not go down the other's socket,
// where it is a message about a session that endpoint has never heard of.
func TestAChangeReachesTheOtherSendersOwnConnection(t *testing.T) {
	s := NewService(NewReceiver("Kitchen"))
	s.Receiver.Player = &player{}

	kitchen := dial(t, s)
	hall := dial(t, s)

	kitchen.send(from("sender-kitchen", NSConnection, Connect()))
	hall.send(from("sender-hall", NSConnection, Connect()))

	// Both connections have to be known before the change, or there is nobody to broadcast to.
	waitFor(t, s, 2)

	kitchen.send(from("sender-kitchen", NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))

	answer := kitchen.expect()
	if answer.Destination != "sender-kitchen" {
		t.Errorf("the answer went to %q", answer.Destination)
	}
	if h, _ := Kind(answer.Payload); h.RequestID != 1 {
		t.Errorf("the answer carries request %d", h.RequestID)
	}

	// And the hall heard about it, on its own connection, with no request id.
	told := hall.expect()
	if told.Destination != "sender-hall" {
		t.Errorf("the broadcast went to %q", told.Destination)
	}
	if h, _ := Kind(told.Payload); h.RequestID != 0 {
		t.Errorf("the broadcast carries request %d, want none", h.RequestID)
	}
}

// waitFor blocks until the service holds a number of connections, so a test does not race the
// goroutine that learns them.
func waitFor(t *testing.T, s *Service, n int) {
	t.Helper()

	deadline := time.Now().Add(patience)
	for time.Now().Before(deadline) {
		if s.Senders() >= n {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("only %d of %d connections were learned", s.Senders(), n)
}

// A sender that pulled the plug has to leave the receiver's list, or every change afterwards builds
// a broadcast to somebody who is not there.
func TestASenderThatWentAwayIsForgotten(t *testing.T) {
	s := NewService(NewReceiver("Kitchen"))

	kitchen := dial(t, s)
	hall := dial(t, s)

	kitchen.send(from("sender-kitchen", NSConnection, Connect()))
	hall.send(from("sender-hall", NSConnection, Connect()))
	waitFor(t, s, 2)

	hall.conn.Close()

	deadline := time.Now().Add(patience)
	for time.Now().Before(deadline) && s.Senders() > 1 {
		time.Sleep(time.Millisecond)
	}
	if got := s.Senders(); got != 1 {
		t.Fatalf("%d connections after one closed", got)
	}

	// The remaining one still works, and gets only its own answer.
	kitchen.send(from("sender-kitchen", NSReceiver, `{"type":"GET_STATUS","requestId":1}`))

	got := kitchen.expect()
	if got.Destination != "sender-kitchen" {
		t.Errorf("came back to %q", got.Destination)
	}
}

// Several messages arriving in one write, which is what a sender that pipelines gives.
func TestSeveralMessagesInOneWrite(t *testing.T) {
	s := NewService(NewReceiver("Kitchen"))
	phone := dial(t, s)

	var buf []byte
	buf = append(buf, Frame(from(SenderID, NSConnection, Connect()))...)
	buf = append(buf, Frame(from(SenderID, NSHeartbeat, Ping()))...)
	buf = append(buf, Frame(from(SenderID, NSHeartbeat, Ping()))...)

	phone.conn.SetWriteDeadline(time.Now().Add(patience))
	if _, err := phone.conn.Write(buf); err != nil {
		t.Fatalf("writing: %v", err)
	}

	for i := range 2 {
		got := phone.expect()
		if h, _ := Kind(got.Payload); h.Type != TypePong {
			t.Errorf("answer %d is %q", i, h.Type)
		}
	}
}

// A message split across writes, which TCP does whether or not it is convenient.
func TestAMessageSplitAcrossWrites(t *testing.T) {
	s := NewService(NewReceiver("Kitchen"))
	phone := dial(t, s)

	framed := Frame(from(SenderID, NSHeartbeat, Ping()))

	go func() {
		phone.conn.SetWriteDeadline(time.Now().Add(patience))
		phone.conn.Write(framed[:3])
		time.Sleep(10 * time.Millisecond)
		phone.conn.SetWriteDeadline(time.Now().Add(patience))
		phone.conn.Write(framed[3:])
	}()

	if h, _ := Kind(phone.expect().Payload); h.Type != TypePong {
		t.Errorf("came back %q", h.Type)
	}
}

// A sender's bad payload is its bug. Dropping the connection over it turns that into a device that
// cannot be cast to.
func TestABadPayloadIsReportedAndTheConnectionKept(t *testing.T) {
	s := NewService(NewReceiver("Kitchen"))

	var mu sync.Mutex
	var faults int
	s.Fault = func(error) { mu.Lock(); faults++; mu.Unlock() }

	phone := dial(t, s)

	phone.send(from(SenderID, NSReceiver, `not json`))
	phone.send(from(SenderID, NSHeartbeat, Ping()))

	// The connection survived, so the ping after the rubbish is still answered.
	if h, _ := Kind(phone.expect().Payload); h.Type != TypePong {
		t.Error("the connection did not survive a bad payload")
	}

	mu.Lock()
	defer mu.Unlock()
	if faults != 1 {
		t.Errorf("%d faults reported, want one", faults)
	}
}

// A length that was wrong once puts every byte after it in the wrong place, so there is nothing to
// carry on from.
func TestAnUnframeableStreamEndsTheConnection(t *testing.T) {
	s := NewService(NewReceiver("Kitchen"))

	ours, theirs := net.Pipe()
	t.Cleanup(func() { ours.Close(); theirs.Close() })

	done := make(chan error, 1)
	go func() { done <- s.Serve(NewConn(theirs)) }()

	// A length inside the cap, then bytes that are not a message.
	ours.SetWriteDeadline(time.Now().Add(patience))
	ours.Write([]byte{0x00, 0x00, 0x00, 0x04, 0xff, 0xff, 0xff, 0xff})

	select {
	case err := <-done:
		if err == nil {
			t.Error("an unframeable stream ended cleanly")
		}
	case <-time.After(patience):
		t.Error("the connection was not closed")
	}
}

// A connection that ends normally is not an error to report.
func TestAClosedConnectionIsNotAFault(t *testing.T) {
	s := NewService(NewReceiver("Kitchen"))

	ours, theirs := net.Pipe()
	t.Cleanup(func() { theirs.Close() })

	done := make(chan error, 1)
	go func() { done <- s.Serve(NewConn(theirs)) }()

	ours.Close()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
			t.Errorf("a closed connection gave %v", err)
		}
	case <-time.After(patience):
		t.Error("serving did not stop")
	}
}

// The end of a track is something nothing asked for, and it has to reach every connection.
func TestFinishingReachesEveryConnection(t *testing.T) {
	s := NewService(NewReceiver("Kitchen"))
	s.Receiver.Player = &player{}

	kitchen := dial(t, s)
	hall := dial(t, s)

	kitchen.send(from("sender-kitchen", NSConnection, Connect()))
	hall.send(from("sender-hall", NSConnection, Connect()))
	waitFor(t, s, 2)

	kitchen.send(from("sender-kitchen", NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))

	// The launch answer to the kitchen, and the broadcast to the hall.
	launched := kitchen.expect()
	hall.expect()

	transport := status(t, launched.Payload).Applications[0].TransportID

	kitchen.send(toApp("sender-kitchen", transport, NSMedia,
		`{"type":"LOAD","requestId":2,"media":{"contentId":"http://example/a.mp3"}}`))
	kitchen.expect()
	hall.expect()

	s.Finished()

	for _, who := range []*sender{kitchen, hall} {
		got := mediaStatus(t, who.expect().Payload)
		if len(got) != 1 {
			t.Fatalf("%d statuses", len(got))
		}
		if got[0].IdleReason != IdleFinished {
			t.Errorf("idle reason %q", got[0].IdleReason)
		}
	}
}

// A sender that has stopped reading must not hold up the device. Its queue fills, and then it is
// dropped rather than blocking whatever was telling it something.
func TestASenderThatStopsReadingIsDroppedRatherThanBlocking(t *testing.T) {
	ours, theirs := net.Pipe()
	t.Cleanup(func() { ours.Close(); theirs.Close() })

	c := NewConn(theirs)

	// Nothing ever reads ours, so the writer blocks on the first message and the queue fills
	// behind it.
	done := make(chan error, 1)
	go func() {
		for range outbound * 4 {
			err := c.Write(Message{
				Destination: "sender-0",
				Namespace:   NSHeartbeat,
				Payload:     Pong(),
			})
			if err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Error("a sender that never read took every message without complaint")
		}
	case <-time.After(patience):
		t.Fatal("writing blocked on a sender that had stopped reading")
	}
}

// The scenario the queue is for: the audio path says a track finished, and must not wait on a
// socket to say so.
func TestFinishingDoesNotWaitOnTheSocket(t *testing.T) {
	s := NewService(NewReceiver("Kitchen"))
	s.Receiver.Player = &player{}

	kitchen := dial(t, s)
	kitchen.send(from("sender-kitchen", NSConnection, Connect()))
	waitFor(t, s, 1)

	kitchen.send(from("sender-kitchen", NSReceiver,
		`{"type":"LAUNCH","requestId":1,"appId":"`+DefaultMediaReceiver+`"}`))
	transport := status(t, kitchen.expect().Payload).Applications[0].TransportID

	kitchen.send(toApp("sender-kitchen", transport, NSMedia,
		`{"type":"LOAD","requestId":2,"media":{"contentId":"http://example/a.mp3"}}`))
	kitchen.expect()

	// Nothing is reading now. Finishing has to return anyway.
	done := make(chan struct{})
	go func() { s.Finished(); close(done) }()

	select {
	case <-done:
	case <-time.After(patience):
		t.Fatal("finishing waited for the socket, which on the device is the audio path waiting")
	}

	// And the message is still delivered once the sender reads again.
	got := mediaStatus(t, kitchen.expect().Payload)
	if len(got) != 1 || got[0].IdleReason != IdleFinished {
		t.Errorf("the status came back %+v", got)
	}
}

// A message for a sender nobody is holding is dropped rather than reported: one that went away
// while a broadcast was being built is ordinary, not a fault.
func TestSendingToNobody(t *testing.T) {
	s := NewService(NewReceiver("Kitchen"))

	var faults int
	s.Fault = func(error) { faults++ }

	s.Send(Message{Destination: "sender-nobody", Namespace: NSReceiver, Payload: Pong()})

	if faults != 0 {
		t.Errorf("%d faults for a sender that is not there", faults)
	}
}
