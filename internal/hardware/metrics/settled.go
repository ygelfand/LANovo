package metrics

import (
	"context"
	"net"
	"time"
)

// Settled is the addresses, once they have stopped changing.
//
// A device comes up with a routable IPv6 address seconds before DHCP answers for IPv4, so anything
// that advertises the moment it has one publishes a record it then has to withdraw. Withdrawing an
// mDNS record is a goodbye packet, and a device that appears, goes and comes back inside a few
// seconds is one a discovering client can be left with a stale view of — which on this device
// looked like Home Assistant not reconnecting after a reboot until its integration was reloaded.
//
// Stable across one interval, which narrows the window rather than closing it: a DHCP server
// slower than the interval still lands after the record is out, and the advertiser republishes as
// it already did. What this removes is the common case, where the two are a second or two apart.
//
// Waiting costs one interval of discovery on a boot that was going to work anyway. Publishing an
// address the device is about to stop having costs a reconnection.
//
// Nil when ctx ends, which is the caller's signal to stop rather than to advertise nothing.
func Settled(ctx context.Context, within time.Duration) []net.IP {
	return settled(ctx, within, Addresses)
}

// settled takes the reader, so a test does not have to own the machine's interfaces.
func settled(ctx context.Context, within time.Duration, read func() []net.IP) []net.IP {
	was := read()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(within):
		}

		now := read()
		if len(now) > 0 && AddressKey(now) == AddressKey(was) {
			return now
		}
		was = now
	}
}
