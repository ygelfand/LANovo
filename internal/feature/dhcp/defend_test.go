package dhcp

import (
	"net"
	"testing"
	"time"
)

var (
	us    = net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}
	them  = net.HardwareAddr{0x02, 0x00, 0x00, 0x00, 0x00, 0x02}
	held  = net.IPv4(192, 168, 1, 40)
	other = net.IPv4(192, 168, 1, 41)
)

// claim is an ARP frame in which sender says it holds addr.
func claim(sender net.HardwareAddr, addr net.IP) []byte {
	return arpAnnounce(sender, addr)
}

// Somebody else saying they hold our address is the conflict RFC 5227 section 2.4 is about.
func TestAnotherHostClaimingOurAddressIsAConflict(t *testing.T) {
	if !conflicting(claim(them, held), us, held) {
		t.Error("another host claiming our address was not a conflict")
	}
}

// Our own announcement comes back on the packet socket. Treating that as a conflict would make the
// device argue with itself, and with the rate limit below, give the address up on the second one.
func TestOurOwnAnnouncementIsNotAConflict(t *testing.T) {
	if conflicting(claim(us, held), us, held) {
		t.Error("the device's own announcement was read as a conflict")
	}
}

func TestAClaimToSomeOtherAddressIsNotAConflict(t *testing.T) {
	if conflicting(claim(them, other), us, held) {
		t.Error("a claim to a different address was a conflict")
	}
}

// A probe has a sender of zero: a host asking whether an address is free, not taking it. Answering
// one is part of probing; it is not the ongoing conflict this watches for.
func TestAProbeForOurAddressIsNotAConflict(t *testing.T) {
	if conflicting(arpProbe(them, held), us, held) {
		t.Error("a probe was read as an ongoing conflict")
	}
}

// Anything that is not an ethernet IPv4 ARP frame is not an opinion about our address.
func TestRubbishIsNotAConflict(t *testing.T) {
	for _, frame := range [][]byte{
		nil,
		make([]byte, 4),
		make([]byte, ethHeader+arpPayload), // all zeroes: not ARP
	} {
		if conflicting(frame, us, held) {
			t.Errorf("a frame of %d bytes was read as a conflict", len(frame))
		}
	}
}

func TestAConflictAgainstNoAddressIsNotOne(t *testing.T) {
	if conflicting(claim(them, held), us, nil) {
		t.Error("a device with no address had one conflicted")
	}
}

// The policy from RFC 5227 section 2.4: answer once, and if the argument carries on inside the
// interval, stop using the address rather than fight over it.
func TestTheFirstConflictIsDefendedAndTheSecondGivesUp(t *testing.T) {
	var d defender
	at := time.Date(2026, time.September, 20, 12, 0, 0, 0, time.UTC)

	if got := d.decide(at); got != defend {
		t.Errorf("the first conflict was answered with %v, want defend", got)
	}
	if got := d.decide(at.Add(time.Second)); got != surrender {
		t.Errorf("a second conflict a second later was answered with %v, want surrender", got)
	}
}

// Past the interval it is a new argument, not a continuation of the old one.
func TestAConflictPastTheIntervalIsDefendedAgain(t *testing.T) {
	var d defender
	at := time.Date(2026, time.September, 20, 12, 0, 0, 0, time.UTC)

	if got := d.decide(at); got != defend {
		t.Fatalf("the first conflict was answered with %v", got)
	}
	if got := d.decide(at.Add(defendInterval + time.Second)); got != defend {
		t.Errorf("a conflict after the interval was answered with %v, want defend", got)
	}
}

// Right on the boundary counts as past it, so a defence every ten seconds forever is possible and a
// pair of hosts trading them as fast as they arrive is not.
func TestTheIntervalBoundaryDefends(t *testing.T) {
	var d defender
	at := time.Date(2026, time.September, 20, 12, 0, 0, 0, time.UTC)

	d.decide(at)

	if got := d.decide(at.Add(defendInterval)); got != defend {
		t.Errorf("a conflict exactly on the interval was answered with %v, want defend", got)
	}
}

// An argument that went quiet does not make the next one a surrender an hour later.
func TestAnArgumentThatSettledIsForgotten(t *testing.T) {
	var d defender
	at := time.Date(2026, time.September, 20, 12, 0, 0, 0, time.UTC)

	d.decide(at)
	d.settled(at.Add(time.Hour))

	if got := d.decide(at.Add(time.Hour)); got != defend {
		t.Errorf("after settling, the next conflict was answered with %v, want defend", got)
	}
}

// Settling inside the interval is not settling: the argument is still going on.
func TestSettlingInsideTheIntervalDoesNotForget(t *testing.T) {
	var d defender
	at := time.Date(2026, time.September, 20, 12, 0, 0, 0, time.UTC)

	d.decide(at)
	d.settled(at.Add(time.Second))

	if got := d.decide(at.Add(2 * time.Second)); got != surrender {
		t.Errorf("a continuing argument was answered with %v, want surrender", got)
	}
}

// What a defence puts on the wire is the announcement, which is what re-asserts the address to
// every cache on the segment.
func TestADefenceIsAnAnnouncementOfOurOwn(t *testing.T) {
	frame := arpAnnounce(us, held)

	got, ok := readARP(frame)
	if !ok {
		t.Fatal("the defence is not a readable ARP frame")
	}
	if got.op != arpRequest {
		t.Errorf("the defence is operation %d, want a request", got.op)
	}
	if !got.senderIP.Equal(held.To4()) {
		t.Errorf("the defence claims %v, want %v", got.senderIP, held)
	}
	if got.senderMAC.String() != us.String() {
		t.Errorf("the defence is from %v, want %v", got.senderMAC, us)
	}

	// And the other side of it: what we send has to read as a conflict to the host we are arguing
	// with, or the two of them never resolve.
	if !conflicting(frame, them, held) {
		t.Error("our defence would not register as a claim on the other host")
	}
}

func TestTheAnswersAreNamed(t *testing.T) {
	for _, at := range []struct {
		a    answer
		want string
	}{{ignore, "ignore"}, {defend, "defend"}, {surrender, "surrender"}} {
		if got := at.a.String(); got != at.want {
			t.Errorf("%d is %q, want %q", at.a, got, at.want)
		}
	}
}
