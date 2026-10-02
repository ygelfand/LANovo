package dhcp

import (
	"fmt"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// releaseMessage is the DHCPRELEASE, as RFC 2131 section 3.1 and table 5 describe it.
//
// The address goes in ciaddr, not in the requested-address option, which is the opposite of a
// decline and for the opposite reason: ciaddr is for an address the client is using, and this is a
// client giving one back. Table 5 says the requested-address option MUST NOT be set here, and a
// server that reads it would be told about an address nobody asked for.
//
// The server identifier is required rather than optional. A release names the server whose lease it
// is, so a network with two servers does not have the other one free an address it never owned.
func releaseMessage(mac net.HardwareAddr, addr, server net.IP) ([]byte, error) {
	if addr.To4() == nil {
		return nil, fmt.Errorf("dhcp: %s is not an IPv4 address", addr)
	}
	if server.To4() == nil {
		return nil, fmt.Errorf("dhcp: no server to release %s to", addr)
	}

	msg, err := dhcpv4.New(
		dhcpv4.WithHwAddr(mac),
		dhcpv4.WithMessageType(dhcpv4.MessageTypeRelease),
		dhcpv4.WithOption(dhcpv4.OptServerIdentifier(server)),
	)
	if err != nil {
		return nil, fmt.Errorf("dhcp: building a release: %w", err)
	}

	msg.ClientIPAddr = addr.To4()

	// No broadcast flag. The client has an address and is unicasting, so there is nothing for a
	// server to broadcast a reply to — and RFC 2131 section 4.4.4 expects no reply at all.
	return msg.ToBytes(), nil
}

// releaseWait is how long the write is given. It is one datagram to a server on the same segment
// and nothing is waited for afterwards, so this only exists so a shutdown cannot hang on a socket.
const releaseWait = 2 * time.Second

// sendRelease gives the lease back so the server can reuse the address rather than holding it until
// it expires.
//
// Over an ordinary UDP socket, unlike the decline beside it. A decline has no address to send from,
// which is what forces it onto a packet socket; a release is sent by a client that still holds its
// address, so the kernel can route it and resolve the server's hardware address the usual way.
//
// Best effort by design. RFC 2131 section 4.4.4 says a server does not reply to a release and a
// client must not expect one, so there is nothing to retry against and nothing to confirm. The
// address is being dropped either way.
func sendRelease(iface string, lease *Lease) error {
	if lease == nil {
		return nil
	}

	link, err := net.InterfaceByName(iface)
	if err != nil {
		return fmt.Errorf("dhcp: %s: %w", iface, err)
	}

	payload, err := releaseMessage(link.HardwareAddr, lease.Address.IP, lease.Server)
	if err != nil {
		return err
	}

	conn, err := dialRelease(lease.Address.IP, lease.Server)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetWriteDeadline(time.Now().Add(releaseWait)); err != nil {
		return fmt.Errorf("dhcp: releasing %s: %w", lease.Address.IP, err)
	}
	if _, err := conn.Write(payload); err != nil {
		return fmt.Errorf("dhcp: releasing %s: %w", lease.Address.IP, err)
	}
	return nil
}

// dialRelease opens the socket the release goes out of.
//
// From the client port when it is free and from whatever is free when it is not. The running client
// may still hold 68, and a release expects no reply, so the source port is not worth failing the
// whole thing over.
func dialRelease(from, to net.IP) (*net.UDPConn, error) {
	server := &net.UDPAddr{IP: to, Port: serverPort}

	conn, err := net.DialUDP("udp4", &net.UDPAddr{IP: from, Port: clientPort}, server)
	if err == nil {
		return conn, nil
	}

	conn, err = net.DialUDP("udp4", &net.UDPAddr{IP: from}, server)
	if err != nil {
		return nil, fmt.Errorf("dhcp: releasing to %s: %w", to, err)
	}
	return conn, nil
}
