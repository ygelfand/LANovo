// Package wpa is wpa_supplicant's control protocol, without a transport.
//
// The supplicant is spoken to two ways here: the installer runs wpa_cli over adb, and the device
// opens the control socket itself. Those differ only in how a command and its reply travel, so
// what a command says and what a reply means live here and each side brings its own Conn.
//
// Nothing here composes a wpa_supplicant.conf. The supplicant owns its configuration and writes it
// itself, which is also why a network added this way survives a reboot.
package wpa

import (
	"crypto/pbkdf2"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// Conn is one command and its reply. Commands are given lowercase, as wpa_cli takes them; a
// transport that needs them another way is the one that knows so.
type Conn interface {
	Cmd(words ...string) (string, error)
}

// Security is how a network is protected.
//
// What this device can join is fixed by its supplicant, which reports key_mgmt as
// "NONE IEEE8021X WPA-EAP WPA-PSK". There is no SAE, so a WPA3-only network cannot be joined at
// all, while one in WPA2/WPA3 transition mode advertises PSK alongside SAE and joins as PSK.
type Security int

const (
	Open Security = iota
	PSK
	SAE
	Enterprise
)

func (s Security) String() string {
	switch s {
	case Open:
		return "open"
	case PSK:
		return "WPA2"
	case SAE:
		return "WPA3"
	}
	return "enterprise"
}

// Supported reports whether this device's supplicant can join.
func (s Security) Supported() bool { return s == Open || s == PSK }

// SecurityOf reads the flags a scan reports. PSK wins over SAE: an access point advertising both is
// in transition mode, and PSK is the half this device can use.
func SecurityOf(flags string) Security {
	switch {
	case strings.Contains(flags, "EAP"):
		return Enterprise
	case strings.Contains(flags, "PSK"):
		return PSK
	case strings.Contains(flags, "SAE"):
		return SAE
	}
	return Open
}

// Network is one access point a scan found.
type Network struct {
	SSID     string
	Signal   int
	Flags    string
	Security Security
}

func (n Network) String() string {
	if !n.Security.Supported() {
		return fmt.Sprintf("%s (%d dBm, %s — not supported)", n.SSID, n.Signal, n.Security)
	}
	return fmt.Sprintf("%s (%d dBm, %s)", n.SSID, n.Signal, n.Security)
}

// ParseScan reads scan_results, keeping the strongest sighting of each name and dropping the
// unnamed ones, which are access points hiding their SSID.
func ParseScan(out string) map[string]Network {
	best := map[string]Network{}
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(fields) < 5 || fields[0] == "bssid / frequency / signal level / flags / ssid" {
			continue
		}

		ssid := fields[4]
		signal, err := strconv.Atoi(fields[2])
		if ssid == "" || err != nil {
			continue
		}
		if seen, ok := best[ssid]; !ok || signal > seen.Signal {
			best[ssid] = Network{SSID: ssid, Signal: signal, Flags: fields[3], Security: SecurityOf(fields[3])}
		}
	}
	return best
}

// ParseNetworks reads list_networks, which is a header and then one tab-separated row per network.
// A row counts only when it starts with an id and carries a name.
func ParseNetworks(out string) []string {
	var names []string
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(fields) < 2 || fields[0] == "network id / ssid / bssid / flags" {
			continue
		}
		if _, err := strconv.Atoi(fields[0]); err == nil && fields[1] != "" {
			names = append(names, fields[1])
		}
	}
	return names
}

// IDs reads list_networks for the ids of every network with this name. Several can share one: a
// join that failed and was left behind is a second row with the same name.
func IDs(out, ssid string) []string {
	var ids []string
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(fields) < 2 || fields[1] != ssid {
			continue
		}
		if _, err := strconv.Atoi(fields[0]); err != nil {
			continue
		}
		ids = append(ids, fields[0])
	}
	return ids
}

// State is what the supplicant says about the connection.
type State struct {
	SSID    string
	State   string
	Address string
}

// Associated reports whether the supplicant got as far as authenticating, which is what says the
// passphrase was right. An address is a separate matter.
func (s State) Associated() bool { return s.State == "COMPLETED" }

// Joined reports whether the device is associated to this network. An empty ssid accepts any, for a
// join where the name was never chosen — WPS.
//
// The name matters because COMPLETED alone is also true of the network the device was already on.
// select_network does not tear the old association down at once, so for a second or two after
// asking to switch, the supplicant still reports the previous network as connected. Judging on
// state alone reports success for the network being left.
func (s State) Joined(ssid string) bool {
	if !s.Associated() {
		return false
	}
	return ssid == "" || s.SSID == ssid
}

func (s State) String() string {
	if s.SSID == "" {
		return s.State
	}
	return fmt.Sprintf("%s, %s, %s", s.SSID, strings.ToLower(s.State), s.Address)
}

// ParseStatus reads the key=value lines status answers with, keeping the three that say what the
// connection is.
func ParseStatus(out string) State {
	var s State
	for line := range strings.SplitSeq(out, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "ssid":
			s.SSID = value
		case "wpa_state":
			s.State = value
		case "ip_address":
			s.Address = value
		}
	}
	return s
}

// Key turns a passphrase into the 256-bit key wpa_supplicant would derive from it anyway: PBKDF2
// over the passphrase with the network name as salt, 4096 rounds. Passing that instead of the
// passphrase keeps the text out of every parser between here and the supplicant, and out of the
// config file it writes.
//
// A 64-character hex string is already a key rather than a passphrase, so it goes through
// untouched.
func Key(ssid, passphrase string) (string, error) {
	if len(passphrase) == 64 && isHex(passphrase) {
		return passphrase, nil
	}
	if len(passphrase) < 8 || len(passphrase) > 63 {
		return "", fmt.Errorf("wpa: a WPA passphrase is 8 to 63 characters, this one is %d", len(passphrase))
	}

	psk, err := pbkdf2.Key(sha1.New, passphrase, []byte(ssid), 4096, 32)
	if err != nil {
		return "", fmt.Errorf("wpa: deriving the key: %w", err)
	}
	return hex.EncodeToString(psk), nil
}

func isHex(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}

// LastLine is the last non-empty line of a reply, which is where wpa_cli puts the answer after its
// own preamble about which interface it selected.
func LastLine(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return lines[len(lines)-1]
}
