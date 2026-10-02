package dhcp

import (
	"net"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

var granted = net.IP{192, 168, 1, 1}

func renewal(t *testing.T) *dhcpv4.DHCPv4 {
	t.Helper()

	msg, err := renewMessage(us, held, false)
	if err != nil {
		t.Fatalf("renewMessage: %v", err)
	}
	return msg
}

func TestARenewalIsARequest(t *testing.T) {
	if got := renewal(t).MessageType(); got != dhcpv4.MessageTypeRequest {
		t.Errorf("a renewal is a %s", got)
	}
}

// What tells a renewal apart from the request that follows an offer. RFC 2131 table 5: ciaddr
// carries an address the client is already using, and the requested-address option is for one it
// has not got yet. A server reading the option would treat this as a new allocation.
func TestTheAddressIsInCiaddrAndNotRequested(t *testing.T) {
	msg := renewal(t)

	if !msg.ClientIPAddr.Equal(held) {
		t.Errorf("ciaddr is %s, want %s", msg.ClientIPAddr, held)
	}
	if got := msg.RequestedIPAddress(); got != nil {
		t.Errorf("the requested-address option is set to %s", got)
	}
}

// RFC 2131 table 5 again: no server identifier on a renewal. Unicast it is already addressed to the
// one server, and broadcast it must not be, or the only server that could answer a rebinding client
// is the one that has stopped answering.
func TestARenewalNamesNoServer(t *testing.T) {
	for _, broadcasting := range []bool{false, true} {
		msg, err := renewMessage(us, held, broadcasting)
		if err != nil {
			t.Fatal(err)
		}
		if got := msg.ServerIdentifier(); got != nil {
			t.Errorf("broadcasting=%v: names server %s", broadcasting, got)
		}
	}
}

// A client that holds an address can be replied to directly, so it does not ask for the answer to
// be broadcast at the whole segment.
func TestARenewalDoesNotAskToBeBroadcastAt(t *testing.T) {
	if renewal(t).IsBroadcast() {
		t.Error("a renewal asked for a broadcast reply")
	}
}

func TestRenewingSomethingThatIsNotIPv4IsRefused(t *testing.T) {
	if _, err := renewMessage(us, net.ParseIP("2001:db8::1"), false); err == nil {
		t.Error("renewing an IPv6 address was allowed")
	}
}

func ack(t *testing.T, opts ...dhcpv4.Modifier) *dhcpv4.DHCPv4 {
	t.Helper()

	msg, err := dhcpv4.New(append([]dhcpv4.Modifier{
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithYourIP(held),
		dhcpv4.WithNetmask(net.IPv4Mask(255, 255, 255, 0)),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(granted)),
	}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

// A lease with no times of its own renews halfway through and rebinds at seven eighths, which is
// what RFC 2131 section 4.4.5 says a client that was told nothing should do.
func TestATimelessLeaseFallsBackToHalfAndSevenEighths(t *testing.T) {
	life := 2 * time.Hour
	got, err := leaseFrom(ack(t, dhcpv4.WithLeaseTime(uint32(life.Seconds()))))
	if err != nil {
		t.Fatal(err)
	}

	within := func(what string, at time.Time, want time.Duration) {
		t.Helper()
		if off := time.Until(at) - want; off < -time.Minute || off > time.Minute {
			t.Errorf("%s is %v away, want about %v", what, time.Until(at).Round(time.Second), want)
		}
	}
	within("renew", got.Renew, life/2)
	within("rebind", got.Rebind, life*7/8)
	within("expiry", got.Expires, life)
}

// Servers are allowed to say anything, and some say T2 at or before T1. Left as given, the renewal
// would find the rebinding deadline already past and broadcast without ever asking the server that
// granted the lease.
func TestARebindTimeThatIsNotAfterRenewIsPutInOrder(t *testing.T) {
	got, err := leaseFrom(ack(t,
		dhcpv4.WithLeaseTime(3600),
		dhcpv4.WithGeneric(dhcpv4.OptionRenewTimeValue, seconds(1800)),
		dhcpv4.WithGeneric(dhcpv4.OptionRebindingTimeValue, seconds(600)),
	))
	if err != nil {
		t.Fatal(err)
	}

	if !got.Rebind.After(got.Renew) {
		t.Errorf("rebind %v is not after renew %v", got.Rebind, got.Renew)
	}
	if !got.Expires.After(got.Rebind) {
		t.Errorf("expiry %v is not after rebind %v", got.Expires, got.Rebind)
	}
}

// A lease with no subnet mask is not one: the rest of it cannot be used without knowing what is
// local, and guessing classful is how a device ends up unable to reach its own gateway.
func TestALeaseWithNoMaskIsRefused(t *testing.T) {
	msg, err := dhcpv4.New(
		dhcpv4.WithMessageType(dhcpv4.MessageTypeAck),
		dhcpv4.WithYourIP(held),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leaseFrom(msg); err == nil {
		t.Error("a lease with no subnet mask was accepted")
	}
}

func seconds(v uint32) []byte {
	return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}
