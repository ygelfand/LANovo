package dhcp

import (
	"context"
	"net"
	"testing"
	"time"
)

// instead swaps the check for one the test can see, and puts the real one back after. Declining is
// swapped out at the same time, so no test puts a packet on the wire.
func instead(t *testing.T, fn func(string, net.IP) (bool, error)) *int {
	t.Helper()

	wasProbe, wasDecline := probing, declining
	t.Cleanup(func() { probing, declining = wasProbe, wasDecline })

	calls := 0
	probing = func(iface string, ip net.IP) (bool, error) {
		calls++
		return fn(iface, ip)
	}
	declining = func(string, net.IP, net.IP, string) error { return nil }
	return &calls
}

// told counts the declines instead of sending them.
func told(t *testing.T) *int {
	t.Helper()

	was := declining
	t.Cleanup(func() { declining = was })

	sent := 0
	declining = func(string, net.IP, net.IP, string) error {
		sent++
		return nil
	}
	return &sent
}

func leaseFor(ip string) *Lease {
	return &Lease{Address: net.IPNet{IP: net.ParseIP(ip), Mask: net.CIDRMask(24, 32)}}
}

// A renewal comes back with the address the device has been using all along. Probing for that
// means listening for an answer this device is the one giving, and costs seconds every time the
// lease comes round.
func TestARenewalOfTheSameAddressIsNotProbed(t *testing.T) {
	calls := instead(t, func(string, net.IP) (bool, error) { return false, nil })

	c := &Client{}
	c.set(leaseFor("10.100.101.106"))

	taken, err := c.contested(context.Background(), leaseFor("10.100.101.106"))
	if err != nil {
		t.Fatal(err)
	}
	if taken {
		t.Error("a renewal was refused")
	}
	if *calls != 0 {
		t.Errorf("the address was probed %d times, want none", *calls)
	}
}

// A different address is a new one, whoever it came from.
func TestANewAddressIsProbed(t *testing.T) {
	calls := instead(t, func(string, net.IP) (bool, error) { return false, nil })

	c := &Client{}
	c.set(leaseFor("10.100.101.106"))

	if _, err := c.contested(context.Background(), leaseFor("10.100.101.50")); err != nil {
		t.Fatal(err)
	}
	if *calls != 1 {
		t.Errorf("the address was probed %d times, want once", *calls)
	}
}

// And the first address of all, where there is nothing held to compare against.
func TestTheFirstAddressIsProbed(t *testing.T) {
	calls := instead(t, func(string, net.IP) (bool, error) { return false, nil })

	if _, err := (&Client{}).contested(context.Background(), leaseFor("10.100.101.106")); err != nil {
		t.Fatal(err)
	}
	if *calls != 1 {
		t.Errorf("the address was probed %d times, want once", *calls)
	}
}

// The whole point: an address somebody else answers for is not taken.
func TestAnAddressSomebodyElseHoldsIsRefused(t *testing.T) {
	instead(t, func(string, net.IP) (bool, error) { return true, nil })

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // so the ten second wait after a refusal does not hold the test up

	taken, err := (&Client{}).contested(ctx, leaseFor("10.100.101.106"))
	if !taken {
		t.Error("an address somebody else holds was taken")
	}
	if err == nil {
		t.Error("the wait after a refusal did not notice the context was done")
	}
}

// Refusing the address protects the network; telling the server is what stops it offering the same
// one straight back, which would turn a conflict into a loop on the backoff.
func TestARefusedAddressIsDeclinedToTheServer(t *testing.T) {
	instead(t, func(string, net.IP) (bool, error) { return true, nil })
	sent := told(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := (&Client{}).contested(ctx, leaseFor("10.100.101.106")); err == nil {
		t.Fatal("the wait after a refusal did not notice the context was done")
	}
	if *sent != 1 {
		t.Errorf("%d declines were sent, want one", *sent)
	}
}

// And nothing is sent for an address that was free, which would tell a server to stop handing out
// an address that is perfectly good.
func TestAnAddressThatIsFreeIsNotDeclined(t *testing.T) {
	instead(t, func(string, net.IP) (bool, error) { return false, nil })
	sent := told(t)

	if _, err := (&Client{}).contested(context.Background(), leaseFor("10.100.101.106")); err != nil {
		t.Fatal(err)
	}
	if *sent != 0 {
		t.Errorf("%d declines were sent for a free address", *sent)
	}
}

// A probe that could not run is not evidence, so there is nothing to tell the server either.
func TestAProbeThatCannotRunDeclinesNothing(t *testing.T) {
	instead(t, func(string, net.IP) (bool, error) {
		return false, net.UnknownNetworkError("no packet socket")
	})
	sent := told(t)

	if _, err := (&Client{}).contested(context.Background(), leaseFor("10.100.101.106")); err != nil {
		t.Fatal(err)
	}
	if *sent != 0 {
		t.Errorf("%d declines were sent after a probe that did not run", *sent)
	}
}

// The probe failing is not the same as the address being taken. A device that refused every
// address it could not check would end up with none at all, which is worse than the duplicate
// this is guarding against.
func TestAProbeThatCannotRunTakesTheAddress(t *testing.T) {
	instead(t, func(string, net.IP) (bool, error) {
		return false, net.UnknownNetworkError("no packet socket")
	})

	taken, err := (&Client{}).contested(context.Background(), leaseFor("10.100.101.106"))
	if err != nil {
		t.Fatalf("a failed probe was reported as a failure to get an address: %v", err)
	}
	if taken {
		t.Error("a failed probe was read as the address being taken")
	}
}

// Ten seconds, which is what RFC 5227 asks for: two hosts fighting over one address should not
// fight fast.
func TestTheWaitAfterARefusalIsRespected(t *testing.T) {
	instead(t, func(string, net.IP) (bool, error) { return true, nil })

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	if _, err := (&Client{}).contested(ctx, leaseFor("10.100.101.106")); err == nil {
		t.Fatal("the wait ended early without the context being done")
	}
	if took := time.Since(start); took < 40*time.Millisecond {
		t.Errorf("came back after %v, so nothing waited", took)
	}
	if backoff != 10*time.Second {
		t.Errorf("the wait is %v, want the ten seconds RFC 5227 asks for", backoff)
	}
}
