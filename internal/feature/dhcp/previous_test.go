package dhcp

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"

	"github.com/ygelfand/LANovo/internal/config"
)

func fresh(t *testing.T) {
	t.Helper()
	config.Use(filepath.Join(t.TempDir(), "state.json"))
}

func what(t *testing.T, want net.IP) *dhcpv4.DHCPv4 {
	t.Helper()

	msg, err := dhcpv4.New(asking(want)...)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	return msg
}

func TestAFreshDeviceAsksForNoAddress(t *testing.T) {
	fresh(t)

	if got := previous(); got != nil {
		t.Errorf("a fresh device remembered %v", got)
	}
	if got := what(t, previous()).RequestedIPAddress(); got != nil {
		t.Errorf("a fresh device asked for %v", got)
	}
}

// RFC 2131 section 4.4.1: a client that had an address names it when it asks for one.
func TestARememberedAddressIsAskedFor(t *testing.T) {
	fresh(t)

	want := net.IPv4(192, 168, 1, 40)
	if err := config.Set().Network().Address(want.String()); err != nil {
		t.Fatalf("remembering: %v", err)
	}

	if got := what(t, previous()).RequestedIPAddress(); !got.Equal(want.To4()) {
		t.Errorf("asked for %v, want %v", got, want)
	}
}

func TestWhatIsAlwaysAskedFor(t *testing.T) {
	fresh(t)

	for _, want := range []net.IP{nil, net.IPv4(192, 168, 1, 40)} {
		msg := what(t, want)

		if msg.HostName() == "" {
			t.Error("the request carries no hostname")
		}
		if !msg.ParameterRequestList().Has(dhcpv4.OptionNTPServers) {
			t.Error("the request does not ask for NTP servers")
		}
	}
}

func TestRubbishInTheFileIsIgnored(t *testing.T) {
	for _, said := range []string{"not an address", "2001:db8::1", "999.1.1.1", " "} {
		fresh(t)

		if err := config.Set().Network().Address(said); err != nil {
			t.Fatalf("writing %q: %v", said, err)
		}
		if got := previous(); got != nil {
			t.Errorf("%q was read as %v", said, got)
		}
	}
}

func TestTheGrantedAddressIsRemembered(t *testing.T) {
	fresh(t)

	remember(net.IPv4(10, 0, 0, 7))

	if got := config.Get().Network.Address; got != "10.0.0.7" {
		t.Errorf("remembered %q, want 10.0.0.7", got)
	}
	if got := previous(); !got.Equal(net.IPv4(10, 0, 0, 7).To4()) {
		t.Errorf("read back %v", got)
	}
}

func TestRememberingTheSameAddressDoesNotRewriteTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	config.Use(path)

	addr := net.IPv4(10, 0, 0, 7)
	remember(addr)

	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the first remember wrote nothing: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	remember(addr)

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("remembering the same address wrote the file again")
	}
}

func TestRememberingADifferentAddressWritesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	config.Use(path)

	remember(net.IPv4(10, 0, 0, 7))

	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("the first remember wrote nothing: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	remember(net.IPv4(10, 0, 0, 8))

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if after.ModTime().Equal(before.ModTime()) {
		t.Error("a new address did not reach the file")
	}
	if got := config.Get().Network.Address; got != "10.0.0.8" {
		t.Errorf("the file says %q, want 10.0.0.8", got)
	}
}

func TestRememberingNothingForgets(t *testing.T) {
	fresh(t)

	remember(net.IPv4(10, 0, 0, 7))
	remember(nil)

	if got := config.Get().Network.Address; got != "" {
		t.Errorf("forgetting left %q", got)
	}
}
