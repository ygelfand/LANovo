package wpa

import (
	"context"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Scanned is what the supplicant can see, strongest first, one entry per name. Several access
// points share a name in any real building, and a name is what a person picks.
func Scanned(ctx context.Context, c Conn) ([]Network, error) {
	// FAIL-BUSY is the supplicant saying it is already scanning, which Android's framework has it
	// doing on its own schedule. The results this waits for are the ones that scan produces.
	if out, err := c.Cmd("scan"); err != nil && !strings.Contains(out, "FAIL-BUSY") {
		return nil, err
	}

	// Results arrive over a few seconds, so the list is read until it stops growing. An empty read
	// is a scan still running rather than an answer, so it keeps waiting.
	var best map[string]Network
	for range 4 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}

		out, err := c.Cmd("scan_results")
		if err != nil {
			return nil, err
		}

		found := ParseScan(out)
		if len(found) > len(best) {
			best = found
			continue
		}
		if len(best) > 0 {
			break
		}
	}

	networks := make([]Network, 0, len(best))
	for _, n := range best {
		networks = append(networks, n)
	}
	slices.SortFunc(networks, func(a, b Network) int { return b.Signal - a.Signal })
	return networks, nil
}

// Configured is the names the supplicant already has.
func Configured(c Conn) ([]string, error) {
	out, err := c.Cmd("list_networks")
	if err != nil {
		return nil, err
	}
	return ParseNetworks(out), nil
}

// Remove forgets every network with this name, so re-joining does not stack duplicates and a failed
// attempt does not stay behind to be retried forever.
func Remove(c Conn, ssid string) error {
	out, err := c.Cmd("list_networks")
	if err != nil {
		return err
	}

	ids := IDs(out, ssid)
	for _, id := range ids {
		if _, err := c.Cmd("remove_network", id); err != nil {
			return err
		}
	}

	if len(ids) == 0 {
		return nil
	}
	_, err = c.Cmd("save_config")
	return err
}

// Add configures a network and returns the supplicant's id for it, without enabling it.
//
// scan_ssid is set for every network, which costs an active probe and is what makes a hidden one
// joinable at all — the only way onto those, since a scan never names them.
func Add(c Conn, ssid, passphrase string) (string, error) {
	out, err := c.Cmd("add_network")
	if err != nil {
		return "", err
	}

	id := strings.TrimSpace(LastLine(out))
	if _, err := strconv.Atoi(id); err != nil {
		return "", fmt.Errorf("wpa: add_network said %q", strings.TrimSpace(out))
	}

	// Both values go over as hex, which wpa_supplicant takes unquoted. A name or passphrase written
	// as a quoted string would cross two parsers on the way — the device shell, then
	// wpa_supplicant's own — and every character that means something to either has to be reasoned
	// about. Hex has no special characters, so there is nothing to reason about.
	set := [][]string{{"ssid", hex.EncodeToString([]byte(ssid))}, {"scan_ssid", "1"}}
	switch {
	case passphrase == "":
		set = append(set, []string{"key_mgmt", "NONE"})
	default:
		psk, err := Key(ssid, passphrase)
		if err != nil {
			_, _ = c.Cmd("remove_network", id)
			return "", err
		}
		set = append(set, []string{"psk", psk})
	}

	for _, kv := range set {
		if _, err := c.Cmd("set_network", id, kv[0], kv[1]); err != nil {
			_, _ = c.Cmd("remove_network", id)
			return "", err
		}
	}
	return id, nil
}

// Join adds a network and selects it. An empty passphrase means an open network.
func Join(c Conn, ssid, passphrase string) error {
	if err := Remove(c, ssid); err != nil {
		return err
	}

	id, err := Add(c, ssid, passphrase)
	if err != nil {
		return err
	}

	for _, cmd := range []string{"enable_network", "select_network"} {
		if _, err := c.Cmd(cmd, id); err != nil {
			_, _ = c.Cmd("remove_network", id)
			return err
		}
	}

	// The supplicant writes its own configuration, so the network survives a reboot and lanovod
	// comes up on it. Assigning an address is somebody else's job.
	_, err = c.Cmd("save_config")
	return err
}

// Reconnect asks the supplicant to join a network it already has: one nothing is driving sits at
// DISCONNECTED until something asks.
//
// REASSOCIATE rather than RECONNECT: RECONNECT only acts on a supplicant that was told to
// DISCONNECT, and answers OK either way.
func Reconnect(c Conn) error {
	for _, cmd := range [][]string{{"enable_network", "all"}, {"reassociate"}} {
		if _, err := c.Cmd(cmd...); err != nil {
			return err
		}
	}
	return nil
}

// WPS joins by push button, for a router that has one: no name to pick and no passphrase to type.
func WPS(c Conn) error {
	if _, err := c.Cmd("wps_pbc"); err != nil {
		return err
	}
	_, err := c.Cmd("save_config")
	return err
}

// Status reads the current connection.
func Status(c Conn) (State, error) {
	out, err := c.Cmd("status")
	if err != nil {
		return State{}, err
	}
	return ParseStatus(out), nil
}

// Wait blocks until the supplicant has joined this network. An empty ssid waits for any network,
// which is all a WPS join can ask for.
func Wait(ctx context.Context, c Conn, ssid string) (State, error) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	var last State
	for {
		s, err := Status(c)
		if err == nil {
			last = s
			if s.Joined(ssid) {
				return s, nil
			}
		}

		select {
		case <-ctx.Done():
			return last, fmt.Errorf("wpa: did not connect (%s): %w", last, ctx.Err())
		case <-tick.C:
		}
	}
}
