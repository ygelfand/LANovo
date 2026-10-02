package cast

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// How a sender finds this device.
//
// A Chromecast answers _googlecast._tcp on the local network and describes itself in the service's
// TXT records. A sender reads those before it connects at all: the name it lists, whether the
// device is busy, and what it can do all come from here rather than from the protocol.
//
// So these are what decides whether the device appears in the cast menu and what it looks like
// there. A record that is missing or malformed is a device that is discovered and then not offered.

// ServiceType is what to advertise on.
const ServiceType = "_googlecast._tcp"

// Port is where a sender connects, and is not negotiable — senders do read it from the service
// record, but every other implementation uses this and a few have it built in.
const Port = 8009

// The capability bits a sender reads out of ca.
//
// The low ones are what they look like. The high bit in the values real devices advertise is not
// documented anywhere findable and is carried rather than invented — see SmartDisplay below.
const (
	CapabilityVideoOut  = 1 << 0
	CapabilityVideoIn   = 1 << 1
	CapabilityAudioOut  = 1 << 2
	CapabilityAudioIn   = 1 << 3
	CapabilityDevMode   = 1 << 4
	CapabilityMultizone = 1 << 5
)

// SmartDisplay is the ca a stock Lenovo Smart Display 10 and a Google Nest Hub both advertise, read off the network.
//
//	bit  value   meaning
//	0    1       video out
//	2    4       audio out
//	9    512     unknown
//	11   2048    unknown
//	15   32768   unknown, absent on audio-only devices
//	16   65536   unknown
//	17   131072  unknown
const SmartDisplay = 231941

// Version is the protocol version in the record. 05 is what current devices say.
const Version = "05"

// Device is what the record describes.
type Device struct {
	// ID is the device's own identifier, thirty two hex characters. Stable across boots, because a
	// sender remembers it — one that changes is a new device in the cast menu every reboot, and the
	// old ones never go away.
	ID string

	// Name is what a person sees.
	Name string

	// Model is what a sender shows under the name.
	Model string

	// Capabilities is the ca value.
	Capabilities int

	// Running is whether an application is up, which senders show as the device being busy.
	Running bool

	// Status is the line under the name — what is playing, or what the device is doing.
	Status string
}

// Identify derives the stable parts of a device from its hardware address.
//
// Derived rather than random, and rather than stored: a device that generates an id at first boot
// has one more file to lose, and losing it means every phone in the house sees a new device and
// keeps the old one in its list for ever. The address is already unique and already survives
// everything.
func Identify(mac, name, model string) Device {
	clean := strings.ReplaceAll(strings.ToLower(mac), ":", "")

	return Device{
		ID:           derive("cast-id", clean),
		Name:         name,
		Model:        model,
		Capabilities: SmartDisplay,
	}
}

// derive makes a stable thirty two character identifier out of a purpose and an address.
//
// The purpose is in the hash so that two identifiers wanted for the same device are not the same
// number, which is what would happen hashing the address on its own.
func derive(purpose, mac string) string {
	sum := sha256.Sum256([]byte(purpose + ":" + mac))
	return hex.EncodeToString(sum[:16])
}

// Records is the TXT records to advertise, in the order real devices publish them.
//
// Order is not meaningful to a resolver, but keeping it makes a capture next to a real device
// readable side by side, which is how a record that is subtly wrong gets found.
func (d Device) Records() []string {
	status := "0"
	if d.Running {
		status = "1"
	}

	return []string{
		"id=" + d.ID,
		"cd=" + d.CloudID(),
		"rm=",
		"ve=" + Version,
		"md=" + d.Model,
		"ic=/setup/icon.png",
		"fn=" + d.Name,
		"ca=" + fmt.Sprint(d.Capabilities),
		"st=" + status,
		"bs=" + d.Proximity(),
		"nf=1",
		"rs=" + d.Status,
	}
}

// CloudID is the cd record and eureka_info's cloud_device_id, uppercase as a stock device has it.
func (d Device) CloudID() string { return strings.ToUpper(derive("cast-cloud", d.ID)) }

// Proximity is the bs record and DEVICE_INFO's wifiProximityId.
func (d Device) Proximity() string { return strings.ToUpper(derive("cast-bootstrap", d.ID))[:12] }

// Instance is the name the service is registered under.
//
// The device's own id, not its friendly name. A friendly name may be anything somebody typed,
// including something that is not a legal DNS label and including the same thing as the device
// next to it — and two services with one instance name is a pair of devices that take turns
// existing.
func (d Device) Instance() string { return d.ID }

// Valid reports what is missing, for a caller that would otherwise advertise a record a sender
// quietly ignores.
func (d Device) Valid() error {
	if len(d.ID) != 32 {
		return fmt.Errorf("cast: the device id is %d characters, want 32", len(d.ID))
	}
	if _, err := hex.DecodeString(d.ID); err != nil {
		return fmt.Errorf("cast: the device id is not hexadecimal")
	}
	if d.Name == "" {
		return fmt.Errorf("cast: a device with no name would be listed as nothing")
	}
	if d.Capabilities == 0 {
		return fmt.Errorf("cast: a device claiming no capabilities is not offered for anything")
	}
	return nil
}
