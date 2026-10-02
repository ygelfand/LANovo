package dhcp

import (
	"bufio"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQuote(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{"a plain argument is left alone", "resolver", "resolver"},
		{"an empty argument is the empty search domain list", "", `""`},
		{"a space would otherwise be two arguments", "a b", `"a b"`},
		{"a quote is escaped", `a"b`, `"a\"b"`},
		{"a backslash is escaped", `a\b`, `"a\\b"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quote(tt.arg); got != tt.want {
				t.Errorf("quote(%q) = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}

// fakeNetd answers one connection with replies, and records what it was asked.
func fakeNetd(t *testing.T, replies ...string) (*netd, <-chan string) {
	t.Helper()

	// Not t.TempDir: it puts the test's name in the path, and a socket path has about a hundred
	// bytes to fit in, which a descriptive name spends on its own.
	dir, err := os.MkdirTemp("", "netd")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	ln, err := net.Listen("unix", filepath.Join(dir, "s"))
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	asked := make(chan string, len(replies))
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		r := bufio.NewReader(conn)
		for _, reply := range replies {
			cmd, err := r.ReadString(0)
			if err != nil {
				return
			}
			asked <- strings.TrimRight(cmd, "\x00")
			conn.Write([]byte(reply))
		}
	}()

	conn, err := net.Dial("unix", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return &netd{conn: conn, r: bufio.NewReader(conn)}, asked
}

func TestCmdFramesWithNUL(t *testing.T) {
	n, asked := fakeNetd(t, "200 1 ok\x00")

	if err := n.cmd("resolver", "setnetdns", "100", "", "10.0.0.1"); err != nil {
		t.Fatalf("cmd: %v", err)
	}

	want := `1 resolver setnetdns 100 "" 10.0.0.1`
	select {
	case got := <-asked:
		if got != want {
			t.Errorf("netd was sent %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("netd was sent nothing")
	}
}

func TestCmdSkipsContinuations(t *testing.T) {
	n, _ := fakeNetd(t, "100 1 working\x00200 1 done\x00")

	if err := n.cmd("network", "create", "100"); err != nil {
		t.Fatalf("cmd: %v", err)
	}
}

func TestCmdReportsFailureCodes(t *testing.T) {
	n, _ := fakeNetd(t, "500 1 Resolver missing arguments\x00")

	err := n.cmd("resolver")
	if err == nil {
		t.Fatal("want an error for a 500")
	}
	if !strings.Contains(err.Error(), "Resolver missing arguments") {
		t.Errorf("error %q does not carry what netd said", err)
	}
}

func TestCmdNumbersEachCommand(t *testing.T) {
	n, asked := fakeNetd(t, "200 1 ok\x00", "200 2 ok\x00")

	for range 2 {
		if err := n.cmd("network", "create", "100"); err != nil {
			t.Fatalf("cmd: %v", err)
		}
	}

	for _, want := range []string{"1 network create 100", "2 network create 100"} {
		select {
		case got := <-asked:
			if got != want {
				t.Errorf("netd was sent %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatal("netd was sent nothing")
		}
	}
}

// Without the on-link route, replies to a machine on our own subnet are sent to the router
// instead of straight to it and never arrive — which reads as a blocked port and is not one.
func TestOnLink(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		mask []byte
		want string
	}{
		{"a /24", "10.100.101.106", []byte{255, 255, 255, 0}, "10.100.101.0/24"},
		{"a /16", "172.16.4.9", []byte{255, 255, 0, 0}, "172.16.0.0/16"},
		{"a /22", "192.168.20.7", []byte{255, 255, 252, 0}, "192.168.20.0/22"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := onLink(net.IPNet{IP: net.ParseIP(tt.ip), Mask: net.IPMask(tt.mask)})
			if got != tt.want {
				t.Errorf("onLink(%s) = %q, want %q", tt.ip, got, tt.want)
			}
		})
	}
}

// A lease with nothing usable gives no route rather than a wrong one.
func TestOnLinkWithoutAMask(t *testing.T) {
	if got := onLink(net.IPNet{IP: net.ParseIP("10.0.0.1")}); got != "" {
		t.Errorf("onLink with no mask gave %q", got)
	}
	if got := onLink(net.IPNet{}); got != "" {
		t.Errorf("onLink of nothing gave %q", got)
	}
}
