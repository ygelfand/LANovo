package metrics

import (
	"context"
	"net"
	"testing"
	"time"
)

// reads hands back a different answer each time it is asked, so a test can play out a boot.
func reads(each ...[]string) func() []net.IP {
	var at int
	return func() []net.IP {
		v := each[min(at, len(each)-1)]
		at++

		out := make([]net.IP, 0, len(v))
		for _, s := range v {
			out = append(out, net.ParseIP(s))
		}
		return out
	}
}

const quick = time.Millisecond

// The case this exists for: IPv6 arrives, DHCP answers a moment later, and what is advertised has
// to be the pair rather than the first of them.
func TestItWaitsForDHCPToAnswer(t *testing.T) {
	read := reads(
		[]string{"fd06::1"},
		[]string{"fd06::1", "10.0.0.5"},
		[]string{"fd06::1", "10.0.0.5"},
	)

	got := settled(context.Background(), quick, read)
	if key := AddressKey(got); key != "10.0.0.5,fd06::1" {
		t.Errorf("settled on %q, want both addresses", key)
	}
}

// A device whose addresses never moved should not be held up beyond the one interval it takes to
// know that.
func TestAStableDeviceSettlesAtOnce(t *testing.T) {
	read := reads([]string{"10.0.0.5"})

	start := time.Now()
	got := settled(context.Background(), 20*time.Millisecond, read)

	if key := AddressKey(got); key != "10.0.0.5" {
		t.Errorf("settled on %q", key)
	}
	if took := time.Since(start); took > 100*time.Millisecond {
		t.Errorf("took %s to settle on an address that never changed", took)
	}
}

// Nothing is not an answer. A device with no address yet keeps waiting rather than advertising an
// empty record.
func TestNoAddressIsNotSettled(t *testing.T) {
	read := reads(
		nil,
		nil,
		nil,
		[]string{"10.0.0.5"},
		[]string{"10.0.0.5"},
	)

	if key := AddressKey(settled(context.Background(), quick, read)); key != "10.0.0.5" {
		t.Errorf("settled on %q, want it to have waited for an address", key)
	}
}

// Ending is a signal to stop, not to advertise nothing.
func TestGivingUpReturnsNothing(t *testing.T) {
	ctx, stop := context.WithCancel(context.Background())
	stop()

	if got := settled(ctx, time.Hour, reads([]string{"10.0.0.5"})); got != nil {
		t.Errorf("returned %v after being told to stop", got)
	}
}

// An address that keeps moving is never settled, which is the point: it must not be published.
func TestAChangingAddressIsNeverSettled(t *testing.T) {
	var at int
	read := func() []net.IP {
		at++
		return []net.IP{net.ParseIP("10.0.0." + string(rune('0'+at%8)))}
	}

	ctx, stop := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer stop()

	if got := settled(ctx, quick, read); got != nil {
		t.Errorf("settled on %v while the address was still moving", got)
	}
}
