package avrcp

import (
	"encoding/binary"
	"fmt"
	"time"
)

// The half of AVRCP that is not a button: what is playing, how far through it is, and being told
// when either changes.
//
// All of it travels inside a vendor-dependent AV/C frame marked with the SIG's company identifier,
// which is how a profile extension gets carried by a command set that had no room for one.

// The pdus this uses. The list is longer; these are what a player needs.
const (
	PDUGetCapabilities      = 0x10
	PDUGetElementAttributes = 0x20
	PDUGetPlayStatus        = 0x30
	PDURegisterNotification = 0x31
	PDUContinue             = 0x40
	PDUAbortContinue        = 0x41
	PDUSetAbsoluteVolume    = 0x50
)

// What can be asked about the current track.
const (
	AttrTitle    = 0x01
	AttrArtist   = 0x02
	AttrAlbum    = 0x03
	AttrTrack    = 0x04
	AttrTotal    = 0x05
	AttrGenre    = 0x06
	AttrDuration = 0x07

	// AttrCoverArt is a handle to fetch the artwork with, over a channel of its own. New in 1.6,
	// and a target that answers a request for everything with the seven above may still hand this
	// one over when it is asked for by number.
	AttrCoverArt = 0x08
)

// The events a controller can register for. Registering is one-shot: the target answers Interim
// straight away and Changed once, and then it has to be registered for again. Forgetting that is
// how metadata updates once and then never again.
const (
	EventPlaybackStatus = 0x01
	EventTrackChanged   = 0x02
	EventTrackEnd       = 0x03
	EventTrackStart     = 0x04
	EventPosition       = 0x05
	EventBattery        = 0x06
	EventSystemStatus   = 0x07
	EventSettings       = 0x08
	EventNowPlaying     = 0x09
	EventAvailPlayers   = 0x0a
	EventAddressPlayer  = 0x0b
	EventUIDs           = 0x0c
	EventVolume         = 0x0d
)

// What the far end is doing.
const (
	StatusStopped = 0x00
	StatusPlaying = 0x01
	StatusPaused  = 0x02
	StatusForward = 0x03
	StatusReverse = 0x04
	StatusError   = 0xff
)

// StatusName is what a playback status is called.
func StatusName(s byte) string {
	switch s {
	case StatusStopped:
		return "stopped"
	case StatusPlaying:
		return "playing"
	case StatusPaused:
		return "paused"
	case StatusForward:
		return "seeking forward"
	case StatusReverse:
		return "seeking back"
	case StatusError:
		return "error"
	}
	return fmt.Sprintf("status %#02x", s)
}

// UTF8 is the character set every phone sends text in, and the only one this asks for.
const UTF8 = 0x006a

// pduHeader is the pdu id, its packet type, and the parameter length.
const pduHeader = 4

// companyBytes is the three-byte company identifier in front of it.
const companyBytes = 3

// PDU is one AVRCP command or response, out of its AV/C wrapper.
type PDU struct {
	ID byte

	// Packet is single, start, continue or end. This is the profile's own fragmentation, separate
	// from AVCTP's and used for the same reason — a response that does not fit. A controller asks
	// for the rest by name with PDUContinue rather than just reading on.
	Packet byte

	Params []byte
}

// ParsePDU reads one out of a vendor-dependent frame's operands.
func ParsePDU(operands []byte) (PDU, error) {
	if len(operands) < companyBytes+pduHeader {
		return PDU{}, ErrShort
	}

	company := uint32(operands[0])<<16 | uint32(operands[1])<<8 | uint32(operands[2])
	if company != SIG {
		return PDU{}, fmt.Errorf("avrcp: company %#06x is not the sig's", company)
	}

	body := operands[companyBytes:]
	n := int(binary.BigEndian.Uint16(body[2:]))
	if len(body) < pduHeader+n {
		return PDU{}, ErrShort
	}

	return PDU{ID: body[0], Packet: body[1], Params: body[pduHeader : pduHeader+n]}, nil
}

// Frame wraps a pdu as the AV/C frame that carries it.
func (p PDU) Frame(code byte) AVC {
	operands := make([]byte, companyBytes+pduHeader+len(p.Params))

	operands[0] = SIG >> 16
	operands[1] = SIG >> 8 & 0xff
	operands[2] = SIG & 0xff
	operands[3] = p.ID
	operands[4] = p.Packet
	binary.BigEndian.PutUint16(operands[5:], uint16(len(p.Params)))
	copy(operands[companyBytes+pduHeader:], p.Params)

	return AVC{Code: code, Subunit: SubunitPanel, Opcode: OpVendorDependent, Operands: operands}
}

// Track is what is playing.
type Track struct {
	Title  string
	Artist string
	Album  string
	Genre  string

	// Number and Total are as the phone sent them, which is text — some send "3", some send "3/12"
	// and some send nothing. Parsing them into numbers loses the cases that are not numbers.
	Number string
	Total  string

	Duration time.Duration

	// Art is the handle the artwork is fetched with, over a channel of its own. Empty where the
	// phone serves no images or has none for this track.
	Art string
}

// Empty reports whether nothing worth showing came back. A phone between tracks answers a metadata
// request with attributes that are all blank, and putting that on the screen replaces a title with
// nothing rather than leaving the last one up.
func (t Track) Empty() bool {
	return t.Title == "" && t.Artist == "" && t.Album == ""
}

// Basics is everything about a track but the artwork, which is asked for on its own: a target
// hands it over only when it is named, and a response carrying all eight is the length that gets
// fragmented.
var Basics = []uint32{
	AttrTitle, AttrArtist, AttrAlbum, AttrTrack, AttrTotal, AttrGenre, AttrDuration,
}

// GetElementAttributes asks what is playing.
//
// The identifier is eight bytes of zero, which means the current track rather than one named in a
// browsing list. Asking for no attributes in particular means all of them.
func GetElementAttributes(attrs ...uint32) PDU {
	params := make([]byte, 8, 9+len(attrs)*4)
	params = append(params, byte(len(attrs)))
	for _, a := range attrs {
		params = binary.BigEndian.AppendUint32(params, a)
	}
	return PDU{ID: PDUGetElementAttributes, Params: params}
}

// ParseElementAttributes reads the answer.
//
// The count is a claim, not a promise: a count past the end of the buffer yields fewer fields
// rather than an error, as bluez does it.
func ParseElementAttributes(params []byte) (Track, error) {
	var t Track

	if len(params) < 1 {
		return t, ErrShort
	}

	count := int(params[0])
	rest := params[1:]

	for range count {
		// Identifier, character set, length.
		const header = 8
		if len(rest) < header {
			break
		}

		id := binary.BigEndian.Uint32(rest)
		n := int(binary.BigEndian.Uint16(rest[6:]))
		if len(rest) < header+n {
			break
		}

		value := string(rest[header : header+n])
		rest = rest[header+n:]

		switch id {
		case AttrTitle:
			t.Title = value
		case AttrArtist:
			t.Artist = value
		case AttrAlbum:
			t.Album = value
		case AttrGenre:
			t.Genre = value
		case AttrTrack:
			t.Number = value
		case AttrTotal:
			t.Total = value
		case AttrDuration:
			t.Duration = parseMillis(value)
		case AttrCoverArt:
			t.Art = value
		}
	}

	return t, nil
}

// parseMillis reads a duration sent as decimal text in milliseconds.
//
// Anything that is not a number comes back as zero rather than an error. A phone that sends "--"
// for a live stream is not malformed, and refusing the whole response over it would lose the title
// as well.
func parseMillis(s string) time.Duration {
	var ms int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		ms = ms*10 + int64(c-'0')

		// Longer than a month is not a track. Stopping here also keeps a long run of digits from
		// wrapping into a negative duration.
		if ms > 30*24*60*60*1000 {
			return 0
		}
	}
	return time.Duration(ms) * time.Millisecond
}

// Progress is where the far end is in the track.
type Progress struct {
	Length   time.Duration
	Position time.Duration
	Status   byte
}

// The continuing-response pdus, for a response too big for one packet.
const (
	PDURequestContinuing = 0x40
	PDUAbortContinuing   = 0x41
)

// AbortContinuing tells the far end to stop sending the rest of a fragmented response.
//
// Rather than reassembling it. bluez does the same and says why: reassembly is not supported, so
// the half already sent is dropped and the exchange is ended cleanly instead of left open.
func AbortContinuing(pdu byte) PDU {
	return PDU{ID: PDUAbortContinuing, Params: []byte{pdu}}
}

// GetPlayStatus asks for it.
func GetPlayStatus() PDU { return PDU{ID: PDUGetPlayStatus} }

// ParsePlayStatus reads the answer.
func ParsePlayStatus(params []byte) (Progress, error) {
	if len(params) < 9 {
		return Progress{}, ErrShort
	}

	return Progress{
		Length:   millis(binary.BigEndian.Uint32(params)),
		Position: millis(binary.BigEndian.Uint32(params[4:])),
		Status:   params[8],
	}, nil
}

// millis turns the profile's milliseconds into a duration.
//
// 0xffffffff is the spec's "not known", which a phone sends for a stream with no end. It is zero
// here rather than forty-nine days, which is what it would otherwise display as.
func millis(v uint32) time.Duration {
	if v == 0xffffffff {
		return 0
	}
	return time.Duration(v) * time.Millisecond
}

// RegisterNotification asks to be told when something changes.
//
// The interval is in seconds and only means anything for position, where it is how often to be
// told. Every other event ignores it.
func RegisterNotification(event byte, interval uint32) PDU {
	params := append([]byte{event}, make([]byte, 4)...)
	binary.BigEndian.PutUint32(params[1:], interval)
	return PDU{ID: PDURegisterNotification, Params: params}
}

// Why a command was refused. A rejection carries one of these as its only parameter, in place of
// whatever the command would have been answered with.
const (
	RefusedBadCommand   = 0x00
	RefusedBadParameter = 0x01
	RefusedBadContent   = 0x02
	RefusedInternal     = 0x03

	// RefusedPlayerMoved is not a refusal. The target answers every outstanding registration with
	// it when the addressed player changes, meaning the registration is void rather than unwanted,
	// and the controller is expected to make it again.
	RefusedPlayerMoved = 0x16
)

// Notification is what a registration came back with.
type Notification struct {
	Event byte

	// Interim is the answer that arrives straight away carrying the current value. The one that
	// arrives later, carrying the new value, is not — and only that one means something changed.
	Interim bool

	// Status is set for a playback status change, Position for a position one, Volume for a volume
	// one. Which is meaningful is Event.
	Status   byte
	Position time.Duration
	Volume   byte

	// Track identifies what is playing for a track change. 0xffffffffffffffff means nothing is.
	Track uint64

	// Player is which player the far end now answers for, on an addressed player change. Zero is
	// no player: the ids a target hands out start at one.
	Player uint16
}

// ParseNotification reads one.
func ParseNotification(params []byte, interim bool) (Notification, error) {
	if len(params) < 1 {
		return Notification{}, ErrShort
	}

	n := Notification{Event: params[0], Interim: interim}
	body := params[1:]

	switch n.Event {
	case EventPlaybackStatus:
		if len(body) < 1 {
			return n, ErrShort
		}
		n.Status = body[0]

	case EventTrackChanged:
		if len(body) < 8 {
			return n, ErrShort
		}
		n.Track = binary.BigEndian.Uint64(body)

	case EventPosition:
		if len(body) < 4 {
			return n, ErrShort
		}
		n.Position = millis(binary.BigEndian.Uint32(body))

	case EventVolume:
		if len(body) < 1 {
			return n, ErrShort
		}
		n.Volume = body[0] & 0x7f

	case EventAddressPlayer:
		// A uid counter follows, which is only good for the browsing channel.
		if len(body) < 2 {
			return n, ErrShort
		}
		n.Player = binary.BigEndian.Uint16(body)
	}

	return n, nil
}

// VolumeMax is the loudest absolute volume, which is seven bits and not a hundred.
const VolumeMax = 0x7f

// Volume turns a percentage into what the profile carries. Anything above a hundred is a hundred:
// a volume that wrapped would be silent at the top of the slider.
func Volume(percent int) byte {
	if percent <= 0 {
		return 0
	}
	if percent >= 100 {
		return VolumeMax
	}
	return byte((percent*VolumeMax + 50) / 100)
}

// Percent turns it back.
func Percent(v byte) int { return (int(v&0x7f)*100 + VolumeMax/2) / VolumeMax }

// SetAbsoluteVolume tells the far end how loud this device now is.
func SetAbsoluteVolume(v byte) PDU {
	return PDU{ID: PDUSetAbsoluteVolume, Params: []byte{v & 0x7f}}
}

// GetCapabilities asks what a target supports. 0x03 is the list of events it will notify about,
// which is worth asking before registering for one that is refused.
const CapabilityEvents = 0x03

func GetCapabilities(what byte) PDU {
	return PDU{ID: PDUGetCapabilities, Params: []byte{what}}
}

// ParseCapabilities reads the event list back.
func ParseCapabilities(params []byte) ([]byte, error) {
	if len(params) < 2 {
		return nil, ErrShort
	}
	if params[0] != CapabilityEvents {
		return nil, fmt.Errorf("avrcp: capability %#02x is not the event list", params[0])
	}

	count := int(params[1])
	if len(params) < 2+count {
		return nil, ErrShort
	}
	return params[2 : 2+count], nil
}
