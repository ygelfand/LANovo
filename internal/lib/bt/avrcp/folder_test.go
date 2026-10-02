package avrcp

import (
	"encoding/binary"
	"testing"
)

// element builds one media element item the way a target writes it.
func element(uid uint64, name string, attrs map[uint32]string) []byte {
	body := binary.BigEndian.AppendUint64(nil, uid)
	body = append(body, 0x00)                          // audio
	body = binary.BigEndian.AppendUint16(body, 0x006a) // utf-8
	body = binary.BigEndian.AppendUint16(body, uint16(len(name)))
	body = append(body, name...)

	body = append(body, byte(len(attrs)))
	for id, v := range attrs {
		body = binary.BigEndian.AppendUint32(body, id)
		body = binary.BigEndian.AppendUint16(body, 0x006a)
		body = binary.BigEndian.AppendUint16(body, uint16(len(v)))
		body = append(body, v...)
	}

	out := []byte{ItemElement}
	out = binary.BigEndian.AppendUint16(out, uint16(len(body)))
	return append(out, body...)
}

// listing wraps items as a whole GetFolderItems answer.
func listing(items ...[]byte) []byte {
	out := []byte{BrowseOK}
	out = binary.BigEndian.AppendUint16(out, 1) // uid counter
	out = binary.BigEndian.AppendUint16(out, uint16(len(items)))
	for _, i := range items {
		out = append(out, i...)
	}
	return out
}

// A now playing list is what says which track comes next, which nothing on the control channel
// reports.
func TestReadingANowPlayingList(t *testing.T) {
	raw := listing(
		element(52, "3AM", map[uint32]string{
			AttrTitle: "3AM", AttrArtist: "Matchbox Twenty", AttrDuration: "225999",
		}),
		element(53, "The Way", map[uint32]string{
			AttrTitle: "The Way", AttrArtist: "Fastball",
		}),
	)

	items, err := ParseFolderItems(raw)
	if err != nil {
		t.Fatalf("ParseFolderItems: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("%d items, want two", len(items))
	}

	if items[0].UID != 52 || items[0].Name != "3AM" {
		t.Errorf("first item is %d %q", items[0].UID, items[0].Name)
	}
	if items[0].Track.Artist != "Matchbox Twenty" {
		t.Errorf("first artist %q", items[0].Track.Artist)
	}
	if got := items[0].Track.Duration.Milliseconds(); got != 225999 {
		t.Errorf("first duration %dms, want 225999", got)
	}
	if items[1].UID != 53 || items[1].Track.Artist != "Fastball" {
		t.Errorf("second item is %d by %q", items[1].UID, items[1].Track.Artist)
	}
}

// Every entry carries its own length, so a kind this does not read is stepped over rather than
// ending the list. A target sends what it likes.
func TestAnUnreadableKindDoesNotEndTheList(t *testing.T) {
	odd := []byte{0x7f, 0x00, 0x03, 0xaa, 0xbb, 0xcc}

	raw := listing(odd, element(9, "After", nil))

	items, err := ParseFolderItems(raw)
	if err != nil {
		t.Fatalf("ParseFolderItems: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("%d items, want the odd one and the one after it", len(items))
	}
	if items[1].Name != "After" {
		t.Errorf("the item after the odd one is %q", items[1].Name)
	}
}

// A refusal is an error rather than an empty list, so a caller does not read "no tracks" as "the
// queue is empty".
func TestARefusedListingIsAnError(t *testing.T) {
	if _, err := ParseFolderItems([]byte{BrowseNoPlayers, 0, 0, 0, 0}); err == nil {
		t.Error("a refused listing came back as an empty one")
	}
}

// A count larger than what arrived yields what is there rather than reading past it.
func TestACountLargerThanTheListing(t *testing.T) {
	raw := listing(element(1, "Only", nil))
	raw[3], raw[4] = 0x00, 0x09 // claim nine

	items, err := ParseFolderItems(raw)
	if err != nil {
		t.Fatalf("ParseFolderItems: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("%d items from a listing claiming nine and carrying one", len(items))
	}
}

func TestAListingShorterThanItsHeader(t *testing.T) {
	if _, err := ParseFolderItems([]byte{BrowseOK, 0x00}); err == nil {
		t.Error("two bytes parsed as a listing")
	}
}

// The request names the scope and an inclusive range, and asks for every attribute.
func TestAskingForARangeOfAList(t *testing.T) {
	b := FolderItems(ScopeNowPlaying, 0, 24)

	if b.ID != BrowseFolderItems {
		t.Fatalf("pdu %#02x, want %#02x", b.ID, BrowseFolderItems)
	}
	if b.Params[0] != ScopeNowPlaying {
		t.Errorf("scope %#02x", b.Params[0])
	}
	if got := binary.BigEndian.Uint32(b.Params[1:]); got != 0 {
		t.Errorf("start %d, want 0", got)
	}
	if got := binary.BigEndian.Uint32(b.Params[5:]); got != 24 {
		t.Errorf("end %d, want 24", got)
	}
	if n := b.Params[9]; n != 0 {
		t.Errorf("asked for %d named attributes, want zero meaning all", n)
	}
}

func TestPointingAtAPlayer(t *testing.T) {
	b := SetBrowsedPlayer(3)

	if b.ID != BrowseSetPlayer {
		t.Fatalf("pdu %#02x, want %#02x", b.ID, BrowseSetPlayer)
	}
	if got := binary.BigEndian.Uint16(b.Params); got != 3 {
		t.Errorf("player %d, want 3", got)
	}
}

// folder builds one folder item, which carries a byte more than an element does before its name.
func folder(uid uint64, name string) []byte {
	body := binary.BigEndian.AppendUint64(nil, uid)
	body = append(body, 0x01)                          // titles
	body = append(body, 0x01)                          // playable
	body = binary.BigEndian.AppendUint16(body, 0x006a) // utf-8
	body = binary.BigEndian.AppendUint16(body, uint16(len(name)))
	body = append(body, name...)

	out := []byte{ItemFolder}
	out = binary.BigEndian.AppendUint16(out, uint16(len(body)))
	return append(out, body...)
}

func TestAFolderIsNamedFromOneByteFurtherIn(t *testing.T) {
	items, err := ParseFolderItems(listing(folder(7, "Albums")))
	if err != nil {
		t.Fatalf("ParseFolderItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("%d items, want 1", len(items))
	}

	if items[0].UID != 7 || items[0].Name != "Albums" {
		t.Errorf("uid %d named %q, want 7 named Albums", items[0].UID, items[0].Name)
	}
}

// The count comes before the range, because a target refuses a range that runs past the end.
func TestCountingAScope(t *testing.T) {
	b := TotalItems(ScopeNowPlaying)

	if b.ID != BrowseTotalItems {
		t.Fatalf("pdu %#02x, want %#02x", b.ID, BrowseTotalItems)
	}
	if b.Params[0] != ScopeNowPlaying {
		t.Errorf("scope %#02x", b.Params[0])
	}

	answer := []byte{BrowseOK, 0x00, 0x01, 0x00, 0x00, 0x00, 0x19}
	n, err := ParseTotalItems(answer)
	if err != nil {
		t.Fatalf("ParseTotalItems: %v", err)
	}
	if n != 25 {
		t.Errorf("counted %d, want 25", n)
	}
}

func TestARefusedCountIsAnError(t *testing.T) {
	if _, err := ParseTotalItems([]byte{BrowseNoPlayers, 0, 0, 0, 0, 0, 0}); err == nil {
		t.Error("a refused count read as a number")
	}
	if _, err := ParseTotalItems([]byte{BrowseOK, 0, 1}); err == nil {
		t.Error("a truncated count read as a number")
	}
}

// The artwork of anything but the playing track is asked for one entry at a time, since a listing
// carries it only for what is playing.
func TestAskingAboutOneEntry(t *testing.T) {
	b := ItemAttributes(ScopeNowPlaying, 4, 2, AttrCoverArt)

	if b.ID != BrowseItemAttrs {
		t.Fatalf("pdu %#02x, want %#02x", b.ID, BrowseItemAttrs)
	}

	want := []byte{
		ScopeNowPlaying,
		0, 0, 0, 0, 0, 0, 0, 4, // uid
		0, 2, // uid counter
		1,          // one attribute
		0, 0, 0, 8, // the artwork
	}
	if len(b.Params) != len(want) {
		t.Fatalf("% x, want % x", b.Params, want)
	}
	for i := range want {
		if b.Params[i] != want[i] {
			t.Fatalf("% x, want % x", b.Params, want)
		}
	}
}

func TestReadingWhatAnEntrySaid(t *testing.T) {
	attrs := []byte{1} // one attribute
	attrs = binary.BigEndian.AppendUint32(attrs, AttrCoverArt)
	attrs = binary.BigEndian.AppendUint16(attrs, 0x006a)
	attrs = binary.BigEndian.AppendUint16(attrs, 7)
	attrs = append(attrs, "2964611"...)

	track, err := ParseItemAttributes(append([]byte{BrowseOK}, attrs...))
	if err != nil {
		t.Fatalf("ParseItemAttributes: %v", err)
	}
	if track.Art != "2964611" {
		t.Errorf("the entry's artwork is %q", track.Art)
	}

	if _, err := ParseItemAttributes([]byte{BrowseBadContent}); err == nil {
		t.Error("a refused entry read as an answer")
	}
}

// The counter a listing came with is what an entry is asked about against.
func TestTheCounterOfAListing(t *testing.T) {
	got, ok := Counter(listing(element(1, "Aja", nil)))
	if !ok {
		t.Fatal("a listing with no counter")
	}
	if got != 1 {
		t.Errorf("counter %d, want 1", got)
	}
}

func TestARefusedPlayerIsAnError(t *testing.T) {
	if err := ParseBrowsedPlayer([]byte{BrowseOK, 0, 1, 0, 0, 0, 3}); err != nil {
		t.Errorf("a player that was accepted: %v", err)
	}
	if err := ParseBrowsedPlayer([]byte{BrowsePlayerMoved}); err == nil {
		t.Error("a refused player read as accepted")
	}
}
