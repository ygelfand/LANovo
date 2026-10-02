//go:build linux

package dhcp

import (
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"time"

	"golang.org/x/sys/unix"
)

// frameSize is the read buffer. An ARP frame is 42 bytes; the rest is room for whatever else the
// socket hands over, since it is bound to ARP but not to this exchange.
const frameSize = 128

// probe asks whether anything else on the segment already holds an address, as RFC 5227 section
// 2.1 has it.
//
// It fails open. A socket that cannot be made, an interface that has gone, a read that errors:
// none of those are evidence of a conflict, and refusing an address because the probe itself broke
// would leave the device with none at all. Only a well formed answer from another machine counts.
func probe(iface string, ip net.IP) (conflict bool, err error) {
	link, err := net.InterfaceByName(iface)
	if err != nil {
		return false, fmt.Errorf("dhcp: %s: %w", iface, err)
	}

	addr := ip.To4()
	if addr == nil {
		return false, fmt.Errorf("dhcp: %s is not an IPv4 address", ip)
	}
	if len(link.HardwareAddr) != 6 {
		return false, fmt.Errorf("dhcp: %s has no ethernet address", iface)
	}

	// Bound to ARP, so the answer to a probe arrives on the socket that sent it. The announcement
	// socket is not bound at all, because nothing answers an announcement.
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ARP)))
	if err != nil {
		return false, fmt.Errorf("dhcp: packet socket: %w", err)
	}
	defer unix.Close(fd)

	bind := &unix.SockaddrLinklayer{Protocol: htons(unix.ETH_P_ARP), Ifindex: link.Index}
	if err := unix.Bind(fd, bind); err != nil {
		return false, fmt.Errorf("dhcp: binding to %s: %w", iface, err)
	}

	to := &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ARP),
		Ifindex:  link.Index,
		Halen:    6,
	}
	copy(to.Addr[:], broadcast)

	frame := arpProbe(link.HardwareAddr, addr)

	// The opening wait is what keeps a roomful of devices coming back from a power cut from all
	// probing on the same tick.
	if listen(fd, link.HardwareAddr, addr, wait(0, probeWait)) {
		return true, nil
	}

	for i := range probeCount {
		if err := unix.Sendto(fd, frame, 0, to); err != nil {
			return false, fmt.Errorf("dhcp: probing for %s: %w", ip, err)
		}

		// After the last probe the wait is the same, and it is the one that decides: an address
		// nobody has claimed by then is free.
		if listen(fd, link.HardwareAddr, addr, wait(probeMin, probeMax)) {
			slog.Warn("something else already holds the offered address", "address", ip, "probe", i+1)
			return true, nil
		}
	}
	return false, nil
}

// listen reads for a while, and reports whether anything said the address was taken.
func listen(fd int, mine net.HardwareAddr, want net.IP, d time.Duration) bool {
	deadline := time.Now().Add(d)
	buf := make([]byte, frameSize)

	for {
		left := time.Until(deadline)
		if left <= 0 {
			return false
		}
		if err := setTimeout(fd, left); err != nil {
			return false
		}

		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			// A timeout is the usual way out, and any other error is not evidence of a conflict.
			return false
		}
		if taken(buf[:n], mine, want) {
			return true
		}
	}
}

func setTimeout(fd int, d time.Duration) error {
	tv := unix.NsecToTimeval(int64(d))
	return unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv)
}

// wait is a random interval in a range, which is what RFC 5227 asks for so two hosts starting
// together do not stay in step.
func wait(low, high time.Duration) time.Duration {
	if high <= low {
		return low
	}
	return low + time.Duration(rand.Int64N(int64(high-low)))
}
