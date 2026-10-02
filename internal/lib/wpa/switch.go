package wpa

import (
	"context"
	"fmt"
	"time"
)

// Settle is how long a switch is given to associate before it is called a failure.
//
// Association is fast when the passphrase is right — a few seconds including the scan for a network
// that was not in the last results. What takes longer than this is a passphrase the access point is
// rejecting, and waiting longer only leaves the device off the network for longer.
const Settle = 20 * time.Second

// Switch moves to another network and comes back if it does not take.
//
// The awkward part of changing networks from the device is that it drops the connection that asked,
// so a wrong passphrase would strand it with no way in. The old network is therefore left
// configured rather than removed, and is selected again when the new one does not associate.
//
// What counts as success is being associated to the new network by name. COMPLETED alone is also
// true of the network being left: select_network does not tear the old association down at once, so
// for a second or two the supplicant still reports the previous one.
//
// An address is not required. Association is what says the passphrase was accepted, and assigning
// an address is somebody else's job that can fail for its own reasons.
func Switch(ctx context.Context, c Conn, ssid, passphrase string) error {
	if ssid == "" {
		return fmt.Errorf("wpa: no network named")
	}

	was, err := Status(c)
	if err != nil {
		return err
	}
	if was.SSID == ssid {
		return fmt.Errorf("wpa: already on %q", ssid)
	}

	// Removed first so a previous attempt at the same name does not stay behind, then added
	// without being selected: nothing is torn down until the new network is fully configured, so a
	// bad passphrase is refused while the device is still on the old network.
	if err := Remove(c, ssid); err != nil {
		return err
	}

	id, err := Add(c, ssid, passphrase)
	if err != nil {
		return err
	}

	if _, err := c.Cmd("enable_network", id); err != nil {
		_, _ = c.Cmd("remove_network", id)
		return err
	}
	if _, err := c.Cmd("select_network", id); err != nil {
		_, _ = c.Cmd("remove_network", id)
		return err
	}

	// select_network disables every other network, which is what makes the switch happen and also
	// what has to be undone on the way back.
	within, stop := context.WithTimeout(ctx, Settle)
	defer stop()

	if _, err := Wait(within, c, ssid); err != nil {
		return back(c, id, was, err)
	}

	_, err = c.Cmd("save_config")
	return err
}

// back forgets the network that did not associate and puts the device on the one it was on.
//
// Every network is enabled again rather than the old one selected by id, because the id was never
// known here and because a device that was on nothing before should be left free to find whatever
// it has. The failure that caused this is what is reported: a device that came back is still a
// device that did not switch.
func back(c Conn, id string, was State, why error) error {
	_, _ = c.Cmd("remove_network", id)
	_, _ = c.Cmd("enable_network", "all")
	_, _ = c.Cmd("reassociate")

	// Not saved. The configuration on disk never had the new network in it, and writing it now
	// would only record the state it already describes.
	if was.SSID == "" {
		return fmt.Errorf("wpa: did not join %w", why)
	}
	return fmt.Errorf("wpa: did not join, back on %q: %w", was.SSID, why)
}
