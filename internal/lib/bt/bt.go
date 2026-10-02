// Package bt is the Bluetooth Classic stack, as the parts between HCI and the speaker.
//
// There is no kernel Bluetooth on this device and no BlueZ — the radio is driven over a UART from
// userspace (internal/hardware/ble), and everything above it has to exist here. As far as can be
// found this is the first pure-Go A2DP sink; the reference implementations are all C, bluekitchen's
// btstack being the readable one.
//
// The layers, bottom up:
//
//	internal/hardware/ble   HCI over the UART, and ACL with its reassembly and credit
//	bt/l2cap                channels: who opens them, what they carry, how wide
//	bt/sdp                  what this device says it is, so a phone finds an audio sink
//	bt/avdtp                what the audio will sound like, and starting it
//	bt/sbc                  turning the result back into samples
//
// Each is bytes in, bytes out, with no hardware on this side of it — which is what makes a stack
// for a device nobody can run tests on testable at all. What none of them do is talk to each other;
// the wiring lives above, and bt_test.go is where the layers are put together and driven the way a
// phone would drive them.
package bt

import (
	"github.com/ygelfand/LANovo/internal/lib/bt/l2cap"
	"github.com/ygelfand/LANovo/internal/lib/bt/sdp"
)

// The L2CAP channels served, and what the service records advertise. The two have to agree: the
// records are how a phone learns where to connect, so one naming a channel the stack does not serve
// is a device that is found and cannot be used.
const (
	SinkPSM    = l2cap.PSMAVDTP
	ControlPSM = l2cap.PSMAVCTP
	BrowsePSM  = l2cap.PSMAVCTPBrowse
)

// RemoteFeatures is what both remote control records claim, which is every category.
//
// The same bits on both on purpose. A phone looking for the generic remote control class matches
// whichever record comes first, and two records disagreeing about what this device can do means
// what it is told depends on the order they were written in. BlueZ claims all four categories on
// both ends for the same reason.
const RemoteFeatures = sdp.CategoryPlayer | sdp.CategoryAmplifier |
	sdp.CategoryTuner | sdp.CategoryMenu

// CoverArt is what the controller adds on top: the three image requests it is willing to make.
//
// Asking is what gets offered. A target hands out the image handle as an extra attribute alongside
// the title, and only to a controller whose record says it would do something with one.
const CoverArt = sdp.FeatureImageProperties | sdp.FeatureImage | sdp.FeatureThumbnail

// Records is what this device answers an SDP search with.
//
// Three of them. The audio sink is the one that matters; the other two are the remote control, as
// its two ends. A device that advertises only the target gets volume and no transport controls, and
// one that advertises only the controller gets the opposite — with nothing on screen to say which
// half is missing.
//
// The handles are arbitrary and only have to be stable within a session and distinct from each
// other, since a phone uses one to ask about the same record twice.
func Records(name string) []sdp.Record {
	return []sdp.Record{
		sdp.AudioSink(0x00010000, name, SinkPSM, sdp.FeatureSpeaker),

		// The controller first. A phone looking up the generic remote control class takes what it
		// finds in the first record carrying it, and only this one can claim cover art: the same
		// bit on a target says images are served here rather than wanted, and a phone that
		// believes it goes looking for a channel that is not published.
		sdp.RemoteControl(0x00010001, name, ControlPSM, BrowsePSM,
			sdp.UUIDAVRemoteControlController, RemoteFeatures|CoverArt),

		sdp.RemoteControl(0x00010002, name, ControlPSM, BrowsePSM,
			sdp.UUIDAVRemoteControlTarget, RemoteFeatures),
	}
}

// Record is the audio sink's, which is the one most things mean.
func Record(name string) sdp.Record { return Records(name)[0] }
