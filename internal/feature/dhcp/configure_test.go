package dhcp

import (
	"fmt"
	"net"
	"strings"
	"testing"
)

// leaseVia is a lease on the same subnet, reached through a given router.
func leaseVia(router string) *Lease {
	return &Lease{
		Address: net.IPNet{IP: net.IPv4(10, 100, 101, 106), Mask: net.CIDRMask(24, 32)},
		Router:  net.ParseIP(router),
		DNS:     []net.IP{net.ParseIP("10.100.101.1")},
	}
}

// spoken collects what netd was told, once per reply the fake was given.
func spoken(t *testing.T, was, l *Lease, replies int) []string {
	t.Helper()

	// Numbered: cmd counts its commands and reads past any reply that is not the one it is owed.
	ok := make([]string, replies)
	for i := range ok {
		ok[i] = fmt.Sprintf("200 %d ok\x00", i+1)
	}

	n, asked := fakeNetd(t, ok...)
	if err := configure(n, "wlan0", was, l); err != nil {
		t.Fatalf("configure: %v", err)
	}

	// Every reply has been consumed by now, so exactly that many commands are waiting.
	said := make([]string, 0, replies)
	for range replies {
		said = append(said, <-asked)
	}
	return said
}

func TestConfigureTakesOutTheOldDefaultWhenTheRouterChanges(t *testing.T) {
	said := spoken(t, leaseVia("10.100.101.1"), leaseVia("10.100.101.254"), 7)

	var removed, added int
	for _, cmd := range said {
		switch {
		case strings.Contains(cmd, "route remove") && strings.Contains(cmd, "10.100.101.1"):
			removed++
		case strings.Contains(cmd, "route add") && strings.Contains(cmd, "10.100.101.254"):
			added++
		}
	}

	if removed != 1 {
		t.Errorf("the old default was removed %d times, want once:\n%s", removed, strings.Join(said, "\n"))
	}
	if added != 1 {
		t.Errorf("the new default was added %d times, want once:\n%s", added, strings.Join(said, "\n"))
	}

	// Order matters: removing after adding would take the new route's place in the table.
	for _, cmd := range said {
		if strings.Contains(cmd, "route add") && strings.Contains(cmd, "0.0.0.0/0") {
			t.Fatal("the default was added before the old one came out")
		}
		if strings.Contains(cmd, "route remove") {
			break
		}
	}
}

func TestConfigureLeavesTheDefaultAloneWhenTheRouterHasNotChanged(t *testing.T) {
	via := "10.100.101.1"
	said := spoken(t, leaseVia(via), leaseVia(via), 6)

	for _, cmd := range said {
		if strings.Contains(cmd, "route remove") {
			t.Fatalf("a route was removed for an unchanged router:\n%s", strings.Join(said, "\n"))
		}
	}
}

func TestConfigureRemovesNothingOnTheFirstLease(t *testing.T) {
	said := spoken(t, nil, leaseVia("10.100.101.1"), 6)

	for _, cmd := range said {
		if strings.Contains(cmd, "route remove") {
			t.Fatalf("a route was removed with no lease before it:\n%s", strings.Join(said, "\n"))
		}
	}
}
