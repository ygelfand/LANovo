package dhcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// errRefused is a server saying no. A NAK ends the lease then and there: the address is not the
// client's any more, so there is nothing left to extend and the only way forward is to start again.
var errRefused = errors.New("dhcp: the server refused the renewal")

// renewMessage is the DHCPREQUEST a client sends to keep the address it already has, as RFC 2131
// section 4.3.2 describes the RENEWING and REBINDING forms.
//
// It is told apart from the request that follows an offer by what is in it rather than by a flag.
// ciaddr carries the address, because the client is using it; the requested-address option MUST NOT
// be set, because that option is for an address the client does not have yet. The server identifier
// is left out too — in RENEWING the message is unicast, so it is already addressed to the one
// server, and in REBINDING any server may answer and naming one would stop the others.
func renewMessage(mac net.HardwareAddr, addr net.IP, broadcasting bool) (*dhcpv4.DHCPv4, error) {
	if addr.To4() == nil {
		return nil, fmt.Errorf("dhcp: %s is not an IPv4 address", addr)
	}

	msg, err := dhcpv4.New(
		dhcpv4.WithHwAddr(mac),
		dhcpv4.WithMessageType(dhcpv4.MessageTypeRequest),
		dhcpv4.WithOption(dhcpv4.OptHostName(hostname())),
		dhcpv4.WithRequestedOptions(dhcpv4.OptionNTPServers),
	)
	if err != nil {
		return nil, fmt.Errorf("dhcp: building a renewal: %w", err)
	}

	msg.ClientIPAddr = addr.To4()

	// The broadcast flag asks the server to reply to everyone, which a client that already holds an
	// address never needs: it can receive a unicast reply, and RFC 2131 section 4.4.5 says a
	// rebinding client broadcasts the request, not that it wants the answer broadcast back.
	msg.SetUnicast()

	return msg, nil
}

// renewWait is how long one attempt waits for an answer before trying again. RFC 2131 retransmits
// at half the remaining time; this is the floor under that, so a lease with hours left does not
// make the first retry hours away.
const renewWait = 10 * time.Second

// extend keeps the address the device already has, rather than negotiating a new one.
//
// Two states, and the only difference between them is who the message goes to. RENEWING unicasts to
// the server that granted the lease, from T1 until T2. REBINDING broadcasts from T2 until the lease
// expires, because a server that has stopped answering is the case that state exists for, and any
// server on the segment may take over.
//
// A refusal is returned as it is. Everything else — a server that says nothing, a socket that will
// not open — is a timeout, and the caller falls back to asking from scratch.
func extend(ctx context.Context, iface string, lease *Lease) (*Lease, error) {
	link, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, fmt.Errorf("dhcp: %s: %w", iface, err)
	}

	for _, phase := range []struct {
		broadcasting bool
		until        time.Time
	}{
		{false, lease.Rebind},
		{true, lease.Expires},
	} {
		for time.Now().Before(phase.until) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}

			got, err := ask(ctx, link, lease, phase.broadcasting)
			switch {
			case errors.Is(err, errRefused):
				return nil, err
			case err == nil:
				return got, nil
			}

			// Half the time that is left, which is RFC 2131 section 4.4.5's schedule, floored so a
			// long lease does not wait hours between two attempts.
			if err := sleep(ctx, max(time.Until(phase.until)/2, renewWait)); err != nil {
				return nil, err
			}
		}
	}
	return nil, fmt.Errorf("dhcp: %s could not be renewed before it expired", lease.Address.IP)
}

// ask sends one renewal and reads the answer.
func ask(ctx context.Context, link *net.Interface, lease *Lease, broadcasting bool) (*Lease, error) {
	msg, err := renewMessage(link.HardwareAddr, lease.Address.IP, broadcasting)
	if err != nil {
		return nil, err
	}

	to := lease.Server
	if broadcasting {
		to = net.IPv4bcast
	}

	conn, err := net.DialUDP("udp4",
		&net.UDPAddr{IP: lease.Address.IP, Port: clientPort},
		&net.UDPAddr{IP: to, Port: serverPort})
	if err != nil {
		return nil, fmt.Errorf("dhcp: renewing to %s: %w", to, err)
	}
	defer conn.Close()

	deadline, ok := ctx.Deadline()
	if soon := time.Now().Add(renewWait); !ok || soon.Before(deadline) {
		deadline = soon
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return nil, err
	}

	if _, err := conn.Write(msg.ToBytes()); err != nil {
		return nil, fmt.Errorf("dhcp: renewing %s: %w", lease.Address.IP, err)
	}

	// Anything else on the port is somebody else's exchange, so reading goes on until the answer
	// to this one arrives or the deadline passes.
	buf := make([]byte, 1500)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return nil, fmt.Errorf("dhcp: waiting for a renewal of %s: %w", lease.Address.IP, err)
		}

		reply, err := dhcpv4.FromBytes(buf[:n])
		if err != nil || reply.TransactionID != msg.TransactionID {
			continue
		}

		switch reply.MessageType() {
		case dhcpv4.MessageTypeNak:
			return nil, errRefused
		case dhcpv4.MessageTypeAck:
			return leaseFrom(reply)
		}
	}
}
