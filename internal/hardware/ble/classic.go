package ble

import (
	"fmt"
	"log/slog"
)

// Being a Bluetooth speaker rather than only a scanner.
//
// Everything else in this package is Low Energy, where the controller answers for itself and
// nothing has to be advertised to be reachable. Classic is the other way round: a phone finds this
// device by inquiry and connects to it by page, and the controller does neither until it is told
// to. A controller that has come up and been left alone is invisible and unconnectable.

// The classic controller commands. Opcode Group Field 3 is the controller and baseband.
const (
	hciWriteLocalName     = 0x0c13
	hciWriteScanEnable    = 0x0c1a
	hciWriteClassOfDevice = 0x0c24
	hciWriteSimplePairing = 0x0c56
	hciWriteEventMask     = 0x0c01
)

// What the controller listens for. Both, so the device can be found and then connected to; a
// controller with only page scan is reachable by something that already knows its address.
const (
	scanOff       = 0x00
	scanInquiry   = 0x01
	scanPage      = 0x02
	scanDiscovery = scanInquiry | scanPage
)

// SpeakerClass is what this device says it is, as the three bytes of a class of device.
//
// Major class audio/video, minor class loudspeaker, with the audio and rendering service bits set.
// It decides the icon a phone draws beside the name and whether it offers to connect for media at
// all, so it is not decoration: a device claiming the wrong class is one a phone will pair with and
// then not send audio to.
//
// Little endian on the wire, so the bytes read minor, major, service. The minor class is the field
// rather than the byte: 0x14 >> 2 is 0x05, loudspeaker.
var SpeakerClass = [3]byte{0x14, 0x04, 0x24}

// discoverable makes the controller findable and connectable under a name.
//
// Named in full each time rather than toggled, because a controller that has been reset is back to
// invisible with no name, and a caller that assumes otherwise is one that stops being a speaker
// after the first hiccup.
func discoverable(s *Session, name string, on bool) error {
	if !on {
		_, err := s.Command(hciWriteScanEnable, scanOff)
		return err
	}

	// Everything the controller reports, including the pairing events, which a controller that has
	// only been reset leaves masked off.
	//
	// Not fatal. The mask a controller comes up with already carries the connection events, so a
	// refusal here costs the simple-pairing ones rather than the link, and saying so beats refusing
	// to be a speaker at all.
	if _, err := s.Command(hciWriteEventMask, eventMask[:]...); err != nil {
		slog.Warn("the controller would not take an event mask, so pairing may not complete",
			"err", err)
	}

	// Secure simple pairing, which every phone since about 2008 expects. Without it the controller
	// asks for a PIN and a modern phone has nowhere to type one.
	if _, err := s.Command(hciWriteSimplePairing, 0x01); err != nil {
		return fmt.Errorf("ble: simple pairing: %w", err)
	}

	if _, err := s.Command(hciWriteClassOfDevice, SpeakerClass[:]...); err != nil {
		return fmt.Errorf("ble: the class of device: %w", err)
	}
	if _, err := s.Command(hciWriteLocalName, localName(name)...); err != nil {
		return fmt.Errorf("ble: the name: %w", err)
	}

	_, err := s.Command(hciWriteScanEnable, scanDiscovery)
	return err
}

// eventMask is every event this device wants, little endian.
//
// The top two bits are reserved and left clear; a mask with them set is one some controllers
// refuse outright.
var eventMask = [8]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x3f}

// nameBytes is the fixed width the name command takes, padded with zeros.
const nameBytes = 248

// localName is a name as the command carries it: 248 bytes, zero padded, cut at a rune rather than
// in the middle of one.
//
// A name is UTF-8 on the wire. Cutting at 248 bytes could leave a partial rune at the end, which a
// phone draws as a replacement character in the name of the thing it is about to pair with.
func localName(name string) []byte {
	out := make([]byte, nameBytes)

	if len(name) <= nameBytes {
		copy(out, name)
		return out
	}

	cut := 0
	for i := range name {
		if i > nameBytes {
			break
		}
		cut = i
	}
	copy(out, name[:cut])
	return out
}

// Buffers is how much the controller will take at once: the size of one ACL packet and how many it
// holds.
//
// Asked rather than assumed. Sending past what it holds is what makes a controller stop answering
// rather than say no, and the numbers differ between chips.
func (r *Radio) Buffers() (size, total int, err error) {
	s, err := r.Line()
	if err != nil {
		return 0, 0, err
	}

	said, err := s.Command(hciReadBufferSize)
	if err != nil {
		return 0, 0, err
	}
	return parseBufferSize(said)
}

// Address is the controller's own, which is what a phone remembers a pairing against.
const hciReadAddress = 0x1009

func (r *Radio) Address() ([6]byte, error) {
	var none [6]byte

	s, err := r.Line()
	if err != nil {
		return none, err
	}

	said, err := s.Command(hciReadAddress)
	if err != nil {
		return none, err
	}
	if len(said) < 6 {
		return none, fmt.Errorf("ble: an address of %d bytes", len(said))
	}

	// Little endian on the wire, as every address in HCI is.
	copy(none[:], said[:6])
	for i, j := 0, 5; i < j; i, j = i+1, j-1 {
		none[i], none[j] = none[j], none[i]
	}
	return none, nil
}
