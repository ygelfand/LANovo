//go:build linux

package dhcp

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/sys/unix"
)

// What RFC 5227 asks for: three announcements, two seconds apart.
const (
	announceCount = 3
	announceEvery = 2 * time.Second
)

// announce broadcasts the device's own address, so everything on the segment learns where it is
// without having to ask.
//
// An ARP request naming the device as both sender and target, which is the announcement form of
// RFC 5227 section 2.3. Every other DHCP client on a network sends these, and a host that does not
// is only reachable by whoever already holds it in cache or whoever it speaks to first — which for
// this device means Home Assistant, connecting inbound, can never reach it after a restart.
//
// Access points care as much as hosts do. One doing proxy ARP answers on behalf of the clients it
// has learned and swallows the broadcast for those it has not, so a client that never announces is
// one the access point cannot answer for.
func announce(iface string, ip net.IP) error {
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

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, 0)
	if err != nil {
		return fmt.Errorf("dhcp: packet socket: %w", err)
	}
	defer unix.Close(fd)

	to := &unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ARP),
		Ifindex:  link.Index,
		Halen:    6,
	}
	copy(to.Addr[:], broadcast)

	frame := arpAnnounce(link.HardwareAddr, addr)
	for i := range announceCount {
		if err := unix.Sendto(fd, frame, 0, to); err != nil {
			return fmt.Errorf("dhcp: announcing %s: %w", ip, err)
		}
		if i < announceCount-1 {
			time.Sleep(announceEvery)
		}
	}
	return nil
}

func htons(v uint16) uint16 { return v<<8 | v>>8 }
