package dhcp

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"
)

// netd holds the resolver bionic asks over /dev/socket/dnsproxyd, and ConnectivityService inside
// system_server is what used to tell it anything. Without that every lookup outside lanovod fails,
// so the lease is handed to netd as well as applied to the interface.
//
// The socket is FrameworkListener's: write "<seq> <command> <args>", read back "<code> <seq> <text>"
// until a code of 200 or more ends the exchange. Both directions are terminated by NUL, not a
// newline. ndc is a client for exactly this.
const (
	netdSocket = "/dev/socket/netd"
	netdReply  = 5 * time.Second

	// netID is what wlan0 is registered under. Android's own networks start at 100.
	netID = "100"
)

type netd struct {
	conn net.Conn
	r    *bufio.Reader
	seq  int
}

func dialNetd() (*netd, error) {
	conn, err := net.Dial("unix", netdSocket)
	if err != nil {
		return nil, fmt.Errorf("netd: %w", err)
	}
	return &netd{conn: conn, r: bufio.NewReader(conn)}, nil
}

func (n *netd) Close() error { return n.conn.Close() }

// cmd sends one command and waits for the code that ends it.
func (n *netd) cmd(args ...string) error {
	if err := n.conn.SetDeadline(time.Now().Add(netdReply)); err != nil {
		return err
	}

	n.seq++
	quoted := make([]string, 0, len(args))
	for _, a := range args {
		quoted = append(quoted, quote(a))
	}
	line := fmt.Sprintf("%d %s\x00", n.seq, strings.Join(quoted, " "))

	if _, err := n.conn.Write([]byte(line)); err != nil {
		return fmt.Errorf("netd: %s: %w", args[0], err)
	}

	for {
		raw, err := n.r.ReadString(0)
		if err != nil {
			return fmt.Errorf("netd: %s: %w", args[0], err)
		}
		reply := strings.TrimSpace(strings.TrimRight(raw, "\x00"))

		fields := strings.SplitN(reply, " ", 3)
		if len(fields) < 2 {
			continue
		}

		// A reply carries the sequence number of the command it answers, so one that arrives late
		// is not read as the answer to whatever was asked next.
		code, err := strconv.Atoi(fields[0])
		if err != nil || code < 200 {
			continue
		}
		if seq, err := strconv.Atoi(fields[1]); err != nil || seq != n.seq {
			continue
		}
		if code >= 400 {
			return fmt.Errorf("netd: %s: %s", strings.Join(args, " "), reply)
		}
		return nil
	}
}

// onLink is the lease's own subnet in CIDR, which is the route to everything reachable without
// going through the router.
func onLink(addr net.IPNet) string {
	ip := addr.IP.To4()
	if ip == nil || addr.Mask == nil {
		return ""
	}
	if _, bits := addr.Mask.Size(); bits != 32 {
		return ""
	}

	subnet := net.IPNet{IP: ip.Mask(addr.Mask), Mask: addr.Mask}
	return subnet.String()
}

// quote wraps what netd's tokenizer would otherwise split or lose.
func quote(arg string) string {
	if arg != "" && !strings.ContainsAny(arg, " \"\\") {
		return arg
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(arg) + `"`
}

// useNetd gives netd the lease, so everything on the device resolving through bionic follows what
// lanovod holds. Re-sent on every lease: the servers and the router change with the network.
//
// was is the lease this one replaces, or nil at startup. A renewal that keeps the address but
// changes the router would otherwise leave the old default route in place beside the new one:
// nothing else removes it, because a kept address keeps everything hanging off it.
func useNetd(iface string, was, l *Lease) error {
	n, err := dialNetd()
	if err != nil {
		return err
	}
	defer n.Close()

	return configure(n, iface, was, l)
}

// configure is the conversation itself, apart from dialing, so a test can hold the other end.
func configure(n *netd, iface string, was, l *Lease) error {
	// Before adding the new one, so the table never holds two defaults. A router reached across a
	// restart is not seen, since nothing remembers what the last one was.
	if was != nil && was.Router != nil && !was.Router.Equal(l.Router) {
		if err := n.cmd("network", "route", "remove", netID, iface,
			"0.0.0.0/0", was.Router.String()); err != nil {
			slog.Warn("the old default route would not come out", "router", was.Router, "err", err)
		}
	}

	// The network and its routes survive a renewal, so they are already there the second time.
	setup := [][]string{
		{"network", "create", netID},
		{"network", "interface", "add", netID, iface},
	}

	// The subnet, on link and with no gateway. Without it every reply to a machine on our own
	// network is sent to the router instead of straight to it, and never arrives — which looks
	// like a firewall dropping the port and is not one.
	if subnet := onLink(l.Address); subnet != "" {
		setup = append(setup, []string{"network", "route", "add", netID, iface, subnet})
	}
	if l.Router != nil {
		setup = append(setup,
			[]string{"network", "route", "add", netID, iface, "0.0.0.0/0", l.Router.String()})
	}
	// Kept rather than returned: these fail from the second lease on, having already been done.
	// They are only worth reporting if what follows also fails.
	var setupErr error
	for _, cmd := range setup {
		setupErr = errors.Join(setupErr, n.cmd(cmd...))
	}

	dns := make([]string, 0, len(l.DNS))
	for _, ip := range l.DNS {
		dns = append(dns, ip.String())
	}
	// Nothing follows to show the setup mattered, so its errors stay unreported.
	if len(dns) == 0 {
		return nil
	}

	if err := n.cmd(append([]string{"resolver", "setnetdns", netID, l.Domain}, dns...)...); err != nil {
		return errors.Join(setupErr, err)
	}
	if err := n.cmd("network", "default", "set", netID); err != nil {
		return errors.Join(setupErr, err)
	}
	return nil
}
