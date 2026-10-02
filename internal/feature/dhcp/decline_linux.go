//go:build linux

package dhcp

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// sendDecline tells the server the address it offered is already in use.
//
// RFC 2131 section 3.1.5. Refusing the address is what protects the network; this is what stops the
// server offering the same one straight back, which without it turns a real conflict into a loop on
// the ten second backoff.
//
// Broadcast, from 0.0.0.0, because the client has no address — that is the whole point — so there
// is nothing for a UDP socket to bind to and the frame goes out whole over a packet socket.
func sendDecline(iface string, addr, server net.IP, why string) error {
	link, err := net.InterfaceByName(iface)
	if err != nil {
		return fmt.Errorf("dhcp: %s: %w", iface, err)
	}
	if len(link.HardwareAddr) != 6 {
		return fmt.Errorf("dhcp: %s has no ethernet address", iface)
	}

	payload, err := declineMessage(link.HardwareAddr, addr, server, why)
	if err != nil {
		return err
	}

	frame := declineFrame(link.HardwareAddr, payload)
	if frame == nil {
		return fmt.Errorf("dhcp: could not build the decline")
	}

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, 0)
	if err != nil {
		return fmt.Errorf("dhcp: packet socket: %w", err)
	}
	defer unix.Close(fd)

	to := &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_IP),
		Ifindex:  link.Index,
		Halen:    6,
	}
	copy(to.Addr[:], broadcast)

	if err := unix.Sendto(fd, frame, 0, to); err != nil {
		return fmt.Errorf("dhcp: declining %s: %w", addr, err)
	}
	return nil
}
