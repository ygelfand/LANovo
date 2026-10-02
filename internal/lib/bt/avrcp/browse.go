package avrcp

import (
	"encoding/binary"
	"fmt"
)

// The browsing channel, on L2CAP 0x001b. Its messages are AVCTP the same as the control channel's;
// what rides inside them is not. There is no AV/C wrapper and no company id — a pdu id, a
// big-endian length, and parameters.
//
// It carries the half of the profile that assumes a phone runs more than one player: which players
// exist, which one is addressed, what is in its folders. A controller that does not speak it is
// told about one implicit player and has no way to ask which one that is.
//
// Both halves are here. A phone that browses this speaker is told there is nothing to browse, which
// is true; the questions this end asks of the phone are what the card's list is made of.

// The browsing pdus.
const (
	BrowseSetPlayer     = 0x70
	BrowseFolderItems   = 0x71
	BrowseChangePath    = 0x72
	BrowseItemAttrs     = 0x73
	BrowseTotalItems    = 0x75
	BrowseSearch        = 0x80
	BrowseAddNowPlaying = 0x90
	BrowseReject        = 0xa0
)

// Where a browsing request is looking.
const (
	ScopePlayers    = 0x00
	ScopeFilesystem = 0x01
	ScopeSearch     = 0x02
	ScopeNowPlaying = 0x03
)

// How a browsing request was answered. Success is 0x04 rather than zero, which is a trap worth
// naming: zero is the code for a command the target does not implement.
const (
	BrowseBadCommand   = 0x00
	BrowseBadParameter = 0x01
	BrowseBadContent   = 0x02
	BrowseInternal     = 0x03
	BrowseOK           = 0x04
	BrowseNoPlayers    = 0x15
	BrowsePlayerMoved  = 0x16
)

// browseHeader is a pdu id and the two length bytes.
const browseHeader = 3

// Browse is one message on the browsing channel.
type Browse struct {
	ID     byte
	Params []byte
}

// ParseBrowse reads one.
//
// The length is checked rather than trusted. A pdu claiming more than arrived is the shape a
// truncated read takes, and reading past it hands the layer above another message's bytes.
func ParseBrowse(buf []byte) (Browse, error) {
	if len(buf) < browseHeader {
		return Browse{}, ErrShort
	}

	n := int(binary.BigEndian.Uint16(buf[1:]))
	if len(buf) < browseHeader+n {
		return Browse{}, fmt.Errorf("avrcp: a browsing pdu says %d bytes and %d arrived",
			n, len(buf)-browseHeader)
	}

	return Browse{ID: buf[0], Params: buf[browseHeader : browseHeader+n]}, nil
}

// Marshal writes it.
func (b Browse) Marshal() []byte {
	out := make([]byte, browseHeader+len(b.Params))
	out[0] = b.ID
	binary.BigEndian.PutUint16(out[1:], uint16(len(b.Params)))
	copy(out[browseHeader:], b.Params)
	return out
}

// Response is it as an AVCTP message answering a message with this label.
func (b Browse) Response(label byte) Transport {
	return Transport{Label: label, Type: MessageResponse, Payload: b.Marshal()}
}

// Reject is the answer to a browsing pdu this side does not implement.
func Reject(status byte) Browse {
	return Browse{ID: BrowseReject, Params: []byte{status}}
}

// What a browsed list holds. A player list gives the first, a folder the second and third.
const (
	ItemPlayer  = 0x01
	ItemFolder  = 0x02
	ItemElement = 0x03
)

// Request is it as an AVCTP message to send, under this label.
func (b Browse) Request(label byte) Transport {
	return Transport{Label: label, Type: MessageCommand, Payload: b.Marshal()}
}

// SetBrowsedPlayer points the browsing channel at a player. Nothing may be listed until it has been
// told which player the listing is of.
func SetBrowsedPlayer(player uint16) Browse {
	params := make([]byte, 2)
	binary.BigEndian.PutUint16(params, player)
	return Browse{ID: BrowseSetPlayer, Params: params}
}

// FolderItems asks for a run of a list, with every attribute of each entry.
//
// The range is inclusive at both ends and a target refuses one that runs past what it holds, so a
// caller that does not know the size asks for a modest window and comes back for more.
func FolderItems(scope byte, start, end uint32) Browse {
	params := make([]byte, 0, 10)
	params = append(params, scope)
	params = binary.BigEndian.AppendUint32(params, start)
	params = binary.BigEndian.AppendUint32(params, end)

	// Zero attributes means every attribute. Naming them instead is how a request ends up carrying
	// whatever the last phone happened to answer with.
	params = append(params, 0)

	return Browse{ID: BrowseFolderItems, Params: params}
}

// ParseBrowsedPlayer reads the answer to pointing the channel at a player. The rest of it describes
// that player's filesystem root, which is a different scope from the one that is playing.
func ParseBrowsedPlayer(params []byte) error {
	if len(params) < 1 {
		return ErrShort
	}
	if status := params[0]; status != BrowseOK {
		return fmt.Errorf("avrcp: a player was refused: %#02x", status)
	}
	return nil
}

// TotalItems asks how many entries a scope holds, which is what a caller needs before asking for a
// range of them.
func TotalItems(scope byte) Browse {
	return Browse{ID: BrowseTotalItems, Params: []byte{scope}}
}

// ParseTotalItems reads the answer.
func ParseTotalItems(params []byte) (uint32, error) {
	const size = 7 // status, uid counter, the count itself
	if len(params) < size {
		return 0, ErrShort
	}
	if status := params[0]; status != BrowseOK {
		return 0, fmt.Errorf("avrcp: counting a scope was refused: %#02x", status)
	}
	return binary.BigEndian.Uint32(params[3:]), nil
}

// ItemAttributes asks about one entry of a scope, which is how anything is learned about a track
// that is not the one playing.
//
// The uid counter is the one the listing came with. A target that has rearranged the scope since
// refuses rather than answering about an entry that has moved.
func ItemAttributes(scope byte, uid uint64, counter uint16, attrs ...uint32) Browse {
	params := make([]byte, 0, 12+len(attrs)*4)
	params = append(params, scope)
	params = binary.BigEndian.AppendUint64(params, uid)
	params = binary.BigEndian.AppendUint16(params, counter)

	params = append(params, byte(len(attrs)))
	for _, a := range attrs {
		params = binary.BigEndian.AppendUint32(params, a)
	}
	return Browse{ID: BrowseItemAttrs, Params: params}
}

// ParseItemAttributes reads the answer, which carries attributes the same way a metadata answer
// does.
func ParseItemAttributes(params []byte) (Track, error) {
	if len(params) < 2 {
		return Track{}, ErrShort
	}
	if status := params[0]; status != BrowseOK {
		return Track{}, fmt.Errorf("avrcp: asking about an entry was refused: %#02x", status)
	}
	return ParseElementAttributes(params[1:])
}

// Counter is the uid counter a listing came with, for asking about what was in it.
func Counter(params []byte) (uint16, bool) {
	if len(params) < 3 {
		return 0, false
	}
	return binary.BigEndian.Uint16(params[1:]), true
}

// Item is one entry of a browsed list.
type Item struct {
	Kind byte

	// UID identifies an entry within its scope, for playing it or asking more about it. Only a
	// folder or an element has one.
	UID uint64

	// Player is which player an entry names, for a player list.
	Player uint16

	// Name is what it is called, and Track the attributes that came with it. A phone sends the
	// title in both; the rest of the attributes are only in Track.
	Name  string
	Track Track
}

// ParseFolderItems reads a folder listing.
//
// Every entry carries its own length, so one of a kind this does not read is stepped over rather
// than ending the list. A target is free to send kinds a controller did not ask about.
func ParseFolderItems(params []byte) ([]Item, error) {
	const header = 5 // status, uid counter
	if len(params) < header {
		return nil, ErrShort
	}

	if status := params[0]; status != BrowseOK {
		return nil, fmt.Errorf("avrcp: a listing was refused: %#02x", status)
	}

	count := int(binary.BigEndian.Uint16(params[3:]))
	rest := params[header:]

	out := make([]Item, 0, count)
	for range count {
		// Kind and the length of everything after it.
		if len(rest) < 3 {
			break
		}

		n := int(binary.BigEndian.Uint16(rest[1:]))
		if len(rest) < 3+n {
			break
		}

		item, body := Item{Kind: rest[0]}, rest[3:3+n]
		rest = rest[3+n:]

		switch item.Kind {
		case ItemFolder, ItemElement:
			// Uid, then a type byte and for a folder whether it is playable, then the name as
			// charset, length and bytes.
			at := 11
			if item.Kind == ItemFolder {
				at = 12
			}
			if len(body) < at+2 {
				continue
			}

			item.UID = binary.BigEndian.Uint64(body)

			length := int(binary.BigEndian.Uint16(body[at:]))
			at += 2
			if len(body) < at+length {
				continue
			}

			item.Name = string(body[at : at+length])
			at += length

			// An element carries its attributes after the name, counted the same way a metadata
			// answer counts them.
			if item.Kind == ItemElement && at < len(body) {
				item.Track, _ = ParseElementAttributes(body[at:])
			}

		case ItemPlayer:
			if len(body) < 2 {
				continue
			}
			item.Player = binary.BigEndian.Uint16(body)

			// Major type, subtype, play status, a sixteen byte feature mask, then the name.
			const named = 2 + 1 + 4 + 1 + 16
			if len(body) < named+4 {
				continue
			}

			length := int(binary.BigEndian.Uint16(body[named+2:]))
			if len(body) < named+4+length {
				continue
			}
			item.Name = string(body[named+4 : named+4+length])
		}

		out = append(out, item)
	}
	return out, nil
}

// Browsed answers one message on the browsing channel.
//
// Every request is rejected, and the status says why in the terms the profile has: a scope this
// side does not keep, or a command it does not implement. A speaker holds no media and addresses no
// player, so there is nothing a phone can ask of it that has an answer other than no.
func Browsed(t Transport) ([]Transport, error) {
	if t.Type != MessageCommand {
		return nil, nil
	}

	b, err := ParseBrowse(t.Payload)
	if err != nil {
		return []Transport{Reject(BrowseBadParameter).Response(t.Label)}, err
	}

	status := byte(BrowseBadCommand)
	switch b.ID {
	case BrowseFolderItems, BrowseTotalItems, BrowseChangePath, BrowseItemAttrs:
		status = BrowseBadParameter
	case BrowseSetPlayer:
		status = BrowseNoPlayers
	}

	return []Transport{Reject(status).Response(t.Label)}, nil
}
