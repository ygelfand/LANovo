package sdp

import (
	"bytes"
	"testing"
)

func sink() Record {
	return AudioSink(0x00010000, "LANovo", UUIDAVDTP, FeatureSpeaker)
}

// What a phone needs to find, in the order it needs it: there is an audio sink, it conforms to
// A2DP, and this is the channel. Miss any and the device either does not appear or appears and
// cannot be connected to.
func TestTheRecordSaysThereIsAnAudioSink(t *testing.T) {
	r := sink()

	classes, ok := r.Attribute(AttrServiceClasses)
	if !ok {
		t.Fatal("the record has no service class list")
	}
	if len(classes.Children) != 1 {
		t.Fatalf("%d service classes, want one", len(classes.Children))
	}
	if v, _ := classes.Children[0].Uint(); v != UUIDAudioSink {
		t.Errorf("the service class is %#x, want an audio sink", v)
	}
}

func TestTheRecordClaimsTheA2DPProfile(t *testing.T) {
	r := sink()

	profiles, ok := r.Attribute(AttrProfiles)
	if !ok {
		t.Fatal("the record has no profile list")
	}
	if len(profiles.Children) != 1 {
		t.Fatalf("%d profiles, want one", len(profiles.Children))
	}

	entry := profiles.Children[0]
	if len(entry.Children) != 2 {
		t.Fatalf("the profile entry has %d parts, want a uuid and a version", len(entry.Children))
	}
	if v, _ := entry.Children[0].Uint(); v != UUIDA2DP {
		t.Errorf("the profile is %#x, want advanced audio distribution", v)
	}
	if v, _ := entry.Children[1].Uint(); v != A2DPVersion {
		t.Errorf("the version is %#x, want %#x", v, A2DPVersion)
	}
}

// The channel number is the part a phone actually connects to, and it is buried two sequences deep
// in the protocol list.
func TestAPhoneCanFindTheChannelToConnectTo(t *testing.T) {
	protocols, ok := sink().Attribute(AttrProtocols)
	if !ok {
		t.Fatal("the record has no protocol list")
	}

	psm, ok := PSM(protocols)
	if !ok {
		t.Fatal("no l2cap channel in the protocol list")
	}
	if psm != UUIDAVDTP {
		t.Errorf("the channel is %#x, want the avdtp one", psm)
	}
}

// A record outside the public browse group exists and is never found, which looks exactly like a
// device that is not there.
func TestTheRecordIsInThePublicBrowseGroup(t *testing.T) {
	groups, ok := sink().Attribute(AttrBrowseGroups)
	if !ok {
		t.Fatal("the record is in no browse group")
	}
	if len(groups.Children) != 1 {
		t.Fatalf("%d browse groups", len(groups.Children))
	}
	if v, _ := groups.Children[0].Uint(); v != UUIDBrowseRoot {
		t.Errorf("the browse group is %#x, want the public root", v)
	}
}

// The spec lets a reader stop once it passes the identifier it wants, so the order is not cosmetic.
func TestTheRecordGoesOutSortedByIdentifier(t *testing.T) {
	// Deliberately out of order.
	r := Record{
		{ID: AttrFeatures, Value: Uint(FeatureSpeaker)},
		{ID: AttrRecordHandle, Value: Uint(1)},
		{ID: AttrName, Value: Text("LANovo")},
	}

	got := r.Element()
	if len(got.Children) != 6 {
		t.Fatalf("%d children, want an id and a value for each of three attributes",
			len(got.Children))
	}

	var last uint32
	for i := 0; i < len(got.Children); i += 2 {
		id, ok := got.Children[i].Uint()
		if !ok {
			t.Fatalf("child %d is not an identifier", i)
		}
		if i > 0 && id <= last {
			t.Errorf("identifier %#x came after %#x", id, last)
		}
		last = id
	}
}

// The whole record has to survive being written and read back, because that is all that happens to
// it: it is built once and sent unchanged.
func TestTheWholeRecordGoesOutAndComesBack(t *testing.T) {
	b, err := sink().Element().Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	got, n, err := ParseElement(b)
	if err != nil {
		t.Fatalf("ParseElement: %v", err)
	}
	if n != len(b) {
		t.Errorf("parsing took %d of %d bytes", n, len(b))
	}
	if len(got.Children) != 14 {
		t.Fatalf("%d children came back, want seven attributes as pairs", len(got.Children))
	}

	// Walk it the way a phone would: find the protocol list by its identifier, then the channel.
	for i := 0; i < len(got.Children); i += 2 {
		id, _ := got.Children[i].Uint()
		if id != AttrProtocols {
			continue
		}
		if psm, ok := PSM(got.Children[i+1]); !ok || psm != UUIDAVDTP {
			t.Errorf("the channel read back as %#x (ok=%v)", psm, ok)
		}
		return
	}
	t.Error("the protocol list was not in the record that came back")
}

func TestTheNameAndFeaturesSurvive(t *testing.T) {
	r := AudioSink(1, "Kitchen", UUIDAVDTP, FeatureSpeaker|FeatureAmplifier)

	name, ok := r.Attribute(AttrName)
	if !ok || string(name.Value) != "Kitchen" {
		t.Errorf("the name came back %q", name.Value)
	}

	features, ok := r.Attribute(AttrFeatures)
	if !ok {
		t.Fatal("the record has no features")
	}
	if v, _ := features.Uint(); v != FeatureSpeaker|FeatureAmplifier {
		t.Errorf("features came back %#x", v)
	}
}

func TestAnAttributeThatIsNotThere(t *testing.T) {
	if _, ok := sink().Attribute(0x1234); ok {
		t.Error("an attribute nobody set was found")
	}
}

// PSM is what a phone uses on a record it did not build, so it has to cope with one that is not
// shaped the way ours is.
func TestFindingTheChannelInSomethingThatIsNotAProtocolList(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   Element
	}{
		{"not a sequence", Uint(1)},
		{"empty", Sequence()},
		{"a layer with no parameter", Sequence(Sequence(UUID16(UUIDL2CAP)))},
		{"no l2cap layer", Sequence(Sequence(UUID16(UUIDAVDTP), Uint(0x0103)))},
	} {
		if psm, ok := PSM(tc.in); ok {
			t.Errorf("%s: found a channel %#x", tc.name, psm)
		}
	}
}

// The widths on the wire, which the spec fixes and a phone reads positionally.
//
// This is what a malformed record looks like from the outside: everything parses, nothing errors,
// and the phone quietly decides the device is not what it claimed. Pinned as bytes rather than as
// values, because the value is right either way and only the width is wrong.
func TestARecordWritesItsFixedWidthsWide(t *testing.T) {
	raw, err := AudioSink(0x00010000, "dev", 0x0019, FeatureSpeaker).Element().Marshal()
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []struct {
		what  string
		bytes []byte
	}{
		// An attribute id is two bytes, so 0x0000 is 09 00 00 rather than 08 00.
		{"the record handle's id", []byte{0x09, 0x00, 0x00}},
		{"the service class list's id", []byte{0x09, 0x00, 0x01}},
		{"the protocol list's id", []byte{0x09, 0x00, 0x04}},

		// A record handle is four.
		{"the handle itself", []byte{0x0a, 0x00, 0x01, 0x00, 0x00}},

		// A PSM is two, inside the L2CAP layer of the protocol list. This is the one a phone
		// connects to, and the reason a narrow one is a device that is found and never used.
		{"L2CAP and its psm", []byte{0x19, 0x01, 0x00, 0x09, 0x00, 0x19}},

		// So is a profile version.
		{"A2DP and its version", []byte{0x19, 0x11, 0x0d, 0x09, 0x01, 0x03}},
	} {
		if !bytes.Contains(raw, want.bytes) {
			t.Errorf("%s is not on the wire as % x", want.what, want.bytes)
		}
	}

	// And nothing anywhere writes an attribute id narrow.
	for id := range 0x20 {
		if bytes.Contains(raw, []byte{0x08, byte(id), 0x0a}) {
			t.Errorf("attribute %#02x is written as one byte", id)
		}
	}
}

// The same for the remote control records, which carry a psm of their own.
func TestARemoteControlRecordWritesItsPSMWide(t *testing.T) {
	raw, err := RemoteControl(1, "dev", 0x0017, 0, UUIDAVRemoteControlTarget, 0x0001).
		Element().Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(raw, []byte{0x19, 0x01, 0x00, 0x09, 0x00, 0x17}) {
		t.Error("the psm is not on the wire as a two byte number")
	}
}
