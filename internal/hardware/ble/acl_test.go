package ble

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

// pdu is an L2CAP PDU of n payload bytes on a channel, which is what reassembly measures against.
func pdu(cid uint16, n int) []byte {
	out := make([]byte, l2capHeader+n)
	binary.LittleEndian.PutUint16(out, uint16(n))
	binary.LittleEndian.PutUint16(out[2:], cid)

	for i := range n {
		out[l2capHeader+i] = byte(i)
	}
	return out
}

func TestFramingAnACLPacketAndReadingItBack(t *testing.T) {
	want := pdu(0x0040, 20)

	frames, err := aclFrames(0x0c5a, want, 64)
	if err != nil {
		t.Fatalf("aclFrames: %v", err)
	}
	if len(frames) != 1 {
		t.Fatalf("a %d byte pdu came out as %d packets, want one", len(want), len(frames))
	}

	got, n, err := parseACL(frames[0])
	if err != nil {
		t.Fatalf("parseACL: %v", err)
	}
	if n != len(frames[0]) {
		t.Errorf("parsing took %d of %d bytes", n, len(frames[0]))
	}
	if got.Handle != 0x0c5a {
		t.Errorf("handle came back %#x", got.Handle)
	}
	if got.Boundary != boundaryStart {
		t.Errorf("boundary came back %#x, want a start", got.Boundary)
	}
	if !bytes.Equal(got.Data, want) {
		t.Error("the payload came back different")
	}
}

// The handle is twelve bits and the flags are the other four of the same word, so a handle near the
// top is where they run into each other.
func TestAHighHandleDoesNotSpillIntoTheFlags(t *testing.T) {
	frames, err := aclFrames(0x0fff, pdu(0x0040, 4), 64)
	if err != nil {
		t.Fatalf("aclFrames: %v", err)
	}

	got, _, err := parseACL(frames[0])
	if err != nil {
		t.Fatalf("parseACL: %v", err)
	}
	if got.Handle != 0x0fff {
		t.Errorf("handle came back %#x, want 0xfff", got.Handle)
	}
	if got.Boundary != boundaryStart || got.Broadcast != 0 {
		t.Errorf("flags came back boundary %#x broadcast %#x, want zero", got.Boundary, got.Broadcast)
	}
}

func TestAHandleWiderThanTwelveBitsIsRefused(t *testing.T) {
	if _, err := aclFrames(0x1000, pdu(0x0040, 4), 64); err == nil {
		t.Error("a thirteen bit handle was accepted")
	}
}

// A message larger than the controller's packet takes several, and only the first says it starts
// one. The far end puts them back together by that flag alone.
func TestALongMessageIsCutIntoPacketsAndPutBackTogether(t *testing.T) {
	for _, mtu := range []int{1, 7, 27, 64, 339, 1021} {
		want := pdu(0x0040, 700)

		frames, err := aclFrames(0x0001, want, mtu)
		if err != nil {
			t.Fatalf("mtu %d: aclFrames: %v", mtu, err)
		}

		r := newReassembler()
		var got []byte

		for i, frame := range frames {
			p, _, err := parseACL(frame)
			if err != nil {
				t.Fatalf("mtu %d: packet %d: %v", mtu, i, err)
			}
			if len(p.Data) > mtu {
				t.Fatalf("mtu %d: packet %d carries %d bytes", mtu, i, len(p.Data))
			}

			first := p.Boundary == boundaryStart
			if first != (i == 0) {
				t.Errorf("mtu %d: packet %d has boundary %#x", mtu, i, p.Boundary)
			}

			done, err := r.add(p)
			if err != nil {
				t.Fatalf("mtu %d: packet %d: %v", mtu, i, err)
			}
			if done != nil {
				got = done
			}
		}

		if !bytes.Equal(got, want) {
			t.Errorf("mtu %d: the message came back as %d bytes, want %d", mtu, len(got), len(want))
		}
		if r.pending() != 0 {
			t.Errorf("mtu %d: %d connections still hold a part message", mtu, r.pending())
		}
	}
}

// Two connections sending at once interleave on the wire, and neither may end up with the other's
// bytes.
func TestTwoConnectionsReassembleApart(t *testing.T) {
	one, two := pdu(0x0040, 200), pdu(0x0041, 200)

	first, _ := aclFrames(0x0001, one, 64)
	second, _ := aclFrames(0x0002, two, 64)

	r := newReassembler()
	var gotOne, gotTwo []byte

	for i := range max(len(first), len(second)) {
		for _, at := range [][2]any{{first, Handle(0x0001)}, {second, Handle(0x0002)}} {
			frames := at[0].([][]byte)
			if i >= len(frames) {
				continue
			}

			p, _, err := parseACL(frames[i])
			if err != nil {
				t.Fatalf("parseACL: %v", err)
			}

			done, err := r.add(p)
			if err != nil {
				t.Fatalf("handle %#x: %v", p.Handle, err)
			}
			if done == nil {
				continue
			}
			if p.Handle == 0x0001 {
				gotOne = done
			} else {
				gotTwo = done
			}
		}
	}

	if !bytes.Equal(gotOne, one) {
		t.Error("the first connection's message came back wrong")
	}
	if !bytes.Equal(gotTwo, two) {
		t.Error("the second connection's message came back wrong")
	}
}

// A first fragment is allowed to be shorter than the L2CAP header, which is the case that catches a
// reassembler that reads the length before it has one.
func TestAFirstFragmentShorterThanTheL2CAPHeader(t *testing.T) {
	want := pdu(0x0040, 40)

	frames, err := aclFrames(0x0003, want, 2)
	if err != nil {
		t.Fatalf("aclFrames: %v", err)
	}

	r := newReassembler()
	var got []byte

	for _, frame := range frames {
		p, _, err := parseACL(frame)
		if err != nil {
			t.Fatalf("parseACL: %v", err)
		}
		done, err := r.add(p)
		if err != nil {
			t.Fatalf("add: %v", err)
		}
		if done != nil {
			got = done
		}
	}

	if !bytes.Equal(got, want) {
		t.Error("a message fragmented two bytes at a time did not come back")
	}
}

// A controller that does not follow the protocol is an error rather than something to paper over:
// stitching two messages together and handing the result up is worse than saying so.
func TestFragmentsThatDoNotMakeSense(t *testing.T) {
	t.Run("continuing nothing", func(t *testing.T) {
		r := newReassembler()
		if _, err := r.add(aclPacket{Handle: 1, Boundary: boundaryContinue, Data: []byte{1}}); err == nil {
			t.Error("a continuation with nothing before it was accepted")
		}
	})

	t.Run("starting twice", func(t *testing.T) {
		r := newReassembler()
		half := pdu(0x0040, 40)[:10]

		if _, err := r.add(aclPacket{Handle: 1, Boundary: boundaryFlushable, Data: half}); err != nil {
			t.Fatalf("the first fragment: %v", err)
		}
		if _, err := r.add(aclPacket{Handle: 1, Boundary: boundaryFlushable, Data: half}); err == nil {
			t.Error("a second start while one was outstanding was accepted")
		}
		if r.pending() != 0 {
			t.Error("the half read message was kept after the error")
		}
	})

	t.Run("more than it said", func(t *testing.T) {
		r := newReassembler()
		long := append(pdu(0x0040, 4), 0xff, 0xff)

		if _, err := r.add(aclPacket{Handle: 1, Boundary: boundaryFlushable, Data: long}); err == nil {
			t.Error("a pdu longer than its own length was accepted")
		}
	})
}

// A link that goes away leaves whatever it was half way through, and the next connection to get
// that handle must not inherit it.
func TestForgettingAConnectionDropsItsHalfMessage(t *testing.T) {
	r := newReassembler()
	half := pdu(0x0040, 40)[:10]

	if _, err := r.add(aclPacket{Handle: 7, Boundary: boundaryFlushable, Data: half}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if r.pending() != 1 {
		t.Fatal("the fragment was not held")
	}

	r.forget(7)

	if r.pending() != 0 {
		t.Error("forgetting the connection left its fragment behind")
	}
	if _, err := r.add(aclPacket{Handle: 7, Boundary: boundaryFlushable, Data: half}); err != nil {
		t.Errorf("the handle could not start again: %v", err)
	}
}

func TestParsingAnACLPacketThatHasNotAllArrived(t *testing.T) {
	frames, _ := aclFrames(0x0001, pdu(0x0040, 40), 64)
	whole := frames[0]

	for n := range len(whole) {
		if _, _, err := parseACL(whole[:n]); !errors.Is(err, errShort) {
			t.Errorf("%d bytes of a %d byte packet gave %v, want a short read", n, len(whole), err)
		}
	}
}

func TestAPacketThatIsNotACLIsRefused(t *testing.T) {
	if _, _, err := parseACL([]byte{typeEvent, 0x0e, 0x04, 1, 2, 3, 4}); err == nil {
		t.Error("an event was read as an acl packet")
	}
}

func TestReadingTheControllersBufferSizes(t *testing.T) {
	// ACL length 1021, SCO length 255, 8 ACL buffers, 8 SCO buffers.
	params := []byte{0xfd, 0x03, 0xff, 0x08, 0x00, 0x08, 0x00}

	size, total, err := parseBufferSize(params)
	if err != nil {
		t.Fatalf("parseBufferSize: %v", err)
	}
	if size != 1021 {
		t.Errorf("packet size came back %d, want 1021", size)
	}
	if total != 8 {
		t.Errorf("buffer count came back %d, want 8", total)
	}
}

func TestBufferSizesThatCannotBeUsed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params []byte
	}{
		{"short", []byte{0xfd, 0x03, 0xff}},
		{"no buffers", []byte{0xfd, 0x03, 0xff, 0x00, 0x00, 0x08, 0x00}},
		{"no room in one", []byte{0x00, 0x00, 0xff, 0x08, 0x00, 0x08, 0x00}},
	} {
		if _, _, err := parseBufferSize(tc.params); err == nil {
			t.Errorf("%s: was accepted", tc.name)
		}
	}
}

func TestReadingBuffersComingBack(t *testing.T) {
	// Two handles: 0x000c finished 3 packets, 0x000d finished 1.
	e := event{Code: eventCompletedPackets, Params: []byte{
		0x02,
		0x0c, 0x00, 0x03, 0x00,
		0x0d, 0x00, 0x01, 0x00,
	}}

	got, err := parseCompletedPackets(e)
	if err != nil {
		t.Fatalf("parseCompletedPackets: %v", err)
	}

	want := []completed{{Handle: 0x0c, Count: 3}, {Handle: 0x0d, Count: 1}}
	if len(got) != len(want) {
		t.Fatalf("%d handles came back, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("handle %d came back %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestCompletedPacketsThatDoNotAddUp(t *testing.T) {
	// Says two handles and carries one.
	e := event{Code: eventCompletedPackets, Params: []byte{0x02, 0x0c, 0x00, 0x03, 0x00}}

	if _, err := parseCompletedPackets(e); err == nil {
		t.Error("an event claiming more handles than it carried was accepted")
	}
	if _, err := parseCompletedPackets(event{Code: eventCommandComplete}); err == nil {
		t.Error("a command complete was read as completed packets")
	}
}

// The whole point of counting: the host stops when the controller is full and goes again when it is
// not. Writing past this is not refused, it is dropped.
func TestCreditStopsAtTheControllersBufferCount(t *testing.T) {
	c, err := newCredit(1021, 3)
	if err != nil {
		t.Fatalf("newCredit: %v", err)
	}

	ctx := context.Background()
	for i := range 3 {
		if err := c.take(ctx); err != nil {
			t.Fatalf("buffer %d: %v", i, err)
		}
	}
	if c.available() != 0 {
		t.Errorf("%d buffers free after taking all three", c.available())
	}

	// A fourth has to wait rather than go through.
	tight, stop := context.WithTimeout(ctx, 50*time.Millisecond)
	defer stop()

	if err := c.take(tight); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("taking a fourth buffer gave %v, want it to wait", err)
	}
}

func TestCreditComesBackAndUnblocksAWaiter(t *testing.T) {
	c, _ := newCredit(1021, 1)
	ctx := context.Background()

	if err := c.take(ctx); err != nil {
		t.Fatalf("the first buffer: %v", err)
	}

	waited := make(chan error, 1)
	go func() {
		got, stop := context.WithTimeout(ctx, 2*time.Second)
		defer stop()
		waited <- c.take(got)
	}()

	// Nothing should get through until the controller says so.
	select {
	case err := <-waited:
		t.Fatalf("a buffer was taken while none were free: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	if err := c.give(1); err != nil {
		t.Fatalf("give: %v", err)
	}

	select {
	case err := <-waited:
		if err != nil {
			t.Errorf("the waiter woke with %v", err)
		}
	case <-time.After(time.Second):
		t.Error("the waiter was not woken by a buffer coming back")
	}
}

// A controller that hands back more than it has is miscounting, and believing it would let the host
// write past what the hardware holds.
func TestMoreCreditThanTheControllerHasIsRefusedAndCapped(t *testing.T) {
	c, _ := newCredit(1021, 4)

	if err := c.take(context.Background()); err != nil {
		t.Fatalf("take: %v", err)
	}

	if err := c.give(9); err == nil {
		t.Error("nine buffers coming back into a pool of four was accepted quietly")
	}
	if c.available() != 4 {
		t.Errorf("%d buffers free, want the ceiling of 4", c.available())
	}
}

func TestACreditPoolTheControllerCannotSupport(t *testing.T) {
	for _, tc := range [][2]int{{0, 8}, {1021, 0}, {-1, 8}} {
		if _, err := newCredit(tc[0], tc[1]); err == nil {
			t.Errorf("a pool of %d buffers of %d bytes was accepted", tc[1], tc[0])
		}
	}
}
