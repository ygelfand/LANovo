package dhcp

import (
	"net"
	"sync"
	"time"
)

// defendInterval is RFC 5227's DEFEND_INTERVAL: the shortest time between two defences of the same
// address. Ten seconds.
const defendInterval = 10 * time.Second

// answer is what to do about a frame claiming our address.
type answer int

const (
	// ignore is everything that is not a conflict, including this device's own traffic.
	ignore answer = iota

	// defend broadcasts an announcement, which re-asserts the address to every cache on the segment
	// and tells the other host it is not free.
	defend

	// surrender is a second conflict inside the interval. RFC 5227 section 2.4: a host that has
	// already defended once and is still being argued with stops using the address rather than
	// fighting over it, because two machines answering for one address is worse for the network
	// than either of them going without.
	surrender
)

func (a answer) String() string {
	switch a {
	case defend:
		return "defend"
	case surrender:
		return "surrender"
	}
	return "ignore"
}

// conflicting reports whether a frame is another host claiming an address as its own.
//
// The sender protocol address is what makes it a claim. A probe for our address has a sender of
// zero and is a host asking rather than taking, which is answered as part of probing and is not a
// conflict; this is the ongoing case from RFC 5227 section 2.4, where somebody is already using it.
func conflicting(frame []byte, mine net.HardwareAddr, ours net.IP) bool {
	ours = ours.To4()
	if ours == nil {
		return false
	}

	got, ok := readARP(frame)
	if !ok || got.ours(mine) {
		return false
	}
	return got.senderIP.Equal(ours)
}

// defender is the policy: answer the first conflict, give up on the second inside the interval.
//
// Rate limited rather than answered every time, because the failure this guards against is two
// hosts each answering the other's announcement as fast as they arrive. A pair doing that fills the
// segment and neither gives way.
type defender struct {
	mu     sync.Mutex
	last   time.Time
	strong bool
}

// decide says what to do about one conflicting frame, and records that it happened.
//
// now is passed rather than read so the policy can be tested without waiting ten seconds for it.
func (d *defender) decide(now time.Time) answer {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.strong && now.Sub(d.last) < defendInterval {
		d.strong = false
		return surrender
	}

	d.last, d.strong = now, true
	return defend
}

// settled forgets a conflict that has gone quiet, so an argument an hour ago does not make the next
// one a surrender.
func (d *defender) settled(now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.strong && now.Sub(d.last) >= defendInterval {
		d.strong = false
	}
}
