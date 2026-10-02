//go:build linux

package dhcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"golang.org/x/sys/unix"
)

// watchPoll is how long a read waits before the loop looks at ctx again. It is not a timeout on
// anything: conflicts are rare and the socket is usually silent, so this only decides how quickly
// the watch notices it has been asked to stop.
const watchPoll = time.Second

// watching answers for the address until ctx ends, and calls lost the moment it is given up.
//
// RFC 5227 section 2.4: a host using an address that another host claims defends it once by
// announcing, and if the argument continues inside DEFEND_INTERVAL it stops using the address
// instead of fighting. Two machines that both defend forever fill the segment and neither is
// reachable, which is worse for everyone than one of them going without.
//
// The defence goes out on the socket that heard the conflict, which is already bound to the right
// interface and is one frame rather than the three an announcement sends.
func watching(ctx context.Context, iface string, ip net.IP, lost func()) error {
	link, err := net.InterfaceByName(iface)
	if err != nil {
		return fmt.Errorf("dhcp: %s: %w", iface, err)
	}

	addr := ip.To4()
	if addr == nil {
		return fmt.Errorf("dhcp: %s is not an IPv4 address", ip)
	}
	if len(link.HardwareAddr) != 6 {
		return fmt.Errorf("dhcp: %s has no ethernet address", iface)
	}

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ARP)))
	if err != nil {
		return fmt.Errorf("dhcp: packet socket: %w", err)
	}
	defer unix.Close(fd)

	if err := unix.Bind(fd, &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ARP),
		Ifindex:  link.Index,
	}); err != nil {
		return fmt.Errorf("dhcp: binding to %s: %w", iface, err)
	}

	to := &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ARP),
		Ifindex:  link.Index,
		Halen:    6,
	}
	copy(to.Addr[:], broadcast)

	frame := arpAnnounce(link.HardwareAddr, addr)
	buf := make([]byte, frameSize)

	var d defender
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		if err := setTimeout(fd, watchPoll); err != nil {
			return fmt.Errorf("dhcp: watching %s: %w", ip, err)
		}

		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			// The poll expiring is how this loop breathes; anything else is the socket going wrong
			// and is worth saying, because a watch that has quietly stopped defends nothing.
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EINTR) {
				continue
			}
			return fmt.Errorf("dhcp: watching %s: %w", ip, err)
		}

		now := time.Now()
		d.settled(now)

		if !conflicting(buf[:n], link.HardwareAddr, addr) {
			continue
		}

		switch got := d.decide(now); got {
		case defend:
			slog.Warn("something else is claiming our address, defending it", "address", ip)
			if err := unix.Sendto(fd, frame, 0, to); err != nil {
				slog.Warn("could not defend the address", "address", ip, "err", err)
			}
		case surrender:
			slog.Warn("giving up an address something else insists on", "address", ip)
			lost()
			return nil
		}
	}
}
