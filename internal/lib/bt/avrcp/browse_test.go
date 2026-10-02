package avrcp

import (
	"bytes"
	"testing"
)

// A browsing pdu is an id, a big-endian length, and that many bytes.
func TestParsingABrowsingPDU(t *testing.T) {
	b, err := ParseBrowse([]byte{BrowseFolderItems, 0x00, 0x03, 0x00, 0x01, 0x02})
	if err != nil {
		t.Fatalf("a well formed pdu did not parse: %v", err)
	}
	if b.ID != BrowseFolderItems {
		t.Errorf("pdu id %#02x, want %#02x", b.ID, BrowseFolderItems)
	}
	if !bytes.Equal(b.Params, []byte{0x00, 0x01, 0x02}) {
		t.Errorf("parameters % #02x, want 00 01 02", b.Params)
	}
}

// The length is big-endian. Read the other way round a three byte pdu claims 768.
func TestTheLengthIsBigEndian(t *testing.T) {
	params := make([]byte, 260)
	whole := Browse{ID: BrowseFolderItems, Params: params}.Marshal()

	if whole[1] != 0x01 || whole[2] != 0x04 {
		t.Fatalf("260 bytes written as % #02x, want 01 04", whole[1:3])
	}

	back, err := ParseBrowse(whole)
	if err != nil {
		t.Fatalf("it did not read back: %v", err)
	}
	if len(back.Params) != 260 {
		t.Errorf("%d parameter bytes back, want 260", len(back.Params))
	}
}

// A pdu claiming more than arrived is refused. Reading to the end of the buffer instead would hand
// the layer above whatever followed.
func TestAPDUThatClaimsMoreThanArrived(t *testing.T) {
	if _, err := ParseBrowse([]byte{BrowseFolderItems, 0x00, 0x20, 0x01}); err == nil {
		t.Error("a pdu claiming 32 bytes with one present was accepted")
	}
}

func TestAPDUShorterThanItsHeader(t *testing.T) {
	if _, err := ParseBrowse([]byte{BrowseFolderItems, 0x00}); err == nil {
		t.Error("two bytes parsed as a pdu")
	}
}

// Every request is answered, so a phone is never left waiting on the channel it opened.
func TestEveryRequestIsAnswered(t *testing.T) {
	for _, id := range []byte{
		BrowseSetPlayer, BrowseFolderItems, BrowseChangePath,
		BrowseItemAttrs, BrowseTotalItems, BrowseSearch, BrowseAddNowPlaying,
	} {
		in := Transport{Label: 3, Type: MessageCommand, Payload: Browse{ID: id}.Marshal()}

		out, _ := Browsed(in)
		if len(out) != 1 {
			t.Fatalf("pdu %#02x got %d answers, want one", id, len(out))
		}
		if out[0].Label != 3 {
			t.Errorf("pdu %#02x answered on label %d, want 3", id, out[0].Label)
		}
		if out[0].Type != MessageResponse {
			t.Errorf("pdu %#02x answered as a command", id)
		}

		b, err := ParseBrowse(out[0].Payload)
		if err != nil {
			t.Fatalf("the answer to %#02x did not parse: %v", id, err)
		}
		if b.ID != BrowseReject {
			t.Errorf("pdu %#02x answered with %#02x, want a reject", id, b.ID)
		}
		if len(b.Params) != 1 {
			t.Fatalf("the reject for %#02x carries %d bytes, want a status", id, len(b.Params))
		}
		if b.Params[0] == BrowseOK {
			t.Errorf("pdu %#02x was rejected with success", id)
		}
	}
}

// A response is not answered. Answering one starts an exchange that never ends.
func TestAResponseIsNotAnswered(t *testing.T) {
	in := Transport{Label: 1, Type: MessageResponse, Payload: Browse{ID: BrowseReject}.Marshal()}

	out, err := Browsed(in)
	if err != nil {
		t.Fatalf("a response was an error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("a response drew %d answers, want none", len(out))
	}
}

// A pdu that does not parse is still answered, because the far end is waiting on the label.
func TestAMalformedPDUIsStillAnswered(t *testing.T) {
	in := Transport{Label: 5, Type: MessageCommand, Payload: []byte{BrowseFolderItems, 0x7f}}

	out, err := Browsed(in)
	if err == nil {
		t.Error("a malformed pdu parsed")
	}
	if len(out) != 1 {
		t.Fatalf("%d answers to a malformed pdu, want one", len(out))
	}
	if out[0].Label != 5 {
		t.Errorf("answered on label %d, want 5", out[0].Label)
	}
}
