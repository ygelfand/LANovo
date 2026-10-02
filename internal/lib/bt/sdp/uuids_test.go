package sdp

import (
	"slices"
	"testing"
)

// A search pattern matches against any UUID in a record, at any depth — not against the service
// class list. Browsing is a search for the public browse group's UUID, and that one is in the
// browse group list, so matching on classes alone gives a device that answers when asked for by
// name and is invisible to anything that goes looking.
func TestARecordsUUIDsAreNotJustItsClasses(t *testing.T) {
	got := AudioSink(1, "Lanovo", 0x0019, FeatureSpeaker).UUIDs()

	for _, want := range []uint32{
		UUIDAudioSink,  // the class list
		UUIDBrowseRoot, // the browse group, which is how a phone enumerates
		UUIDL2CAP,      // nested two deep in the protocol list
		UUIDAVDTP,
		UUIDA2DP, // nested in the profile list
	} {
		if !slices.Contains(got, want) {
			t.Errorf("%#x is in the record and not in its uuids: %#x", want, got)
		}
	}
}

// Numbers that are not UUIDs must not come back as them. A channel number or a version collected
// as a UUID makes a record match searches for services it does not offer.
func TestNumbersAreNotCollectedAsUUIDs(t *testing.T) {
	got := AudioSink(1, "Lanovo", 0x0019, FeatureSpeaker).UUIDs()

	// The record handle, the feature bits and the profile version are all plain integers.
	for _, wrong := range []uint32{1, uint32(FeatureSpeaker), A2DPVersion} {
		if slices.Contains(got, wrong) {
			t.Errorf("%#x was collected as a uuid", wrong)
		}
	}
}

func TestTheRemoteControlRecordsCarryTheirClasses(t *testing.T) {
	target := RemoteControl(2, "Lanovo", 0x0017, 0, UUIDAVRemoteControlTarget, CategoryAmplifier)

	got := target.UUIDs()
	for _, want := range []uint32{UUIDAVRemoteControlTarget, UUIDAVRemoteControl, UUIDAVCTP} {
		if !slices.Contains(got, want) {
			t.Errorf("%#x is missing from %#x", want, got)
		}
	}

	// The controller is not also the target: a device claiming both classes on one record tells a
	// phone it can be controlled and can control, from the same end.
	controller := RemoteControl(3, "Lanovo", 0x0017, 0x001b, UUIDAVRemoteControlController, CategoryPlayer)
	if slices.Contains(controller.UUIDs(), uint32(UUIDAVRemoteControlTarget)) {
		t.Error("the controller record claims to be the target as well")
	}

	// Both records carry the generic class. A phone searching for that rather than for either end
	// finds nothing without it.
	if !slices.Contains(controller.UUIDs(), uint32(UUIDAVRemoteControl)) {
		t.Error("the controller record does not carry the generic remote control class")
	}
}

// The browsing channel's number travels with the bit that claims it. A record saying it browses
// and not saying where is one a phone believes and cannot act on.
func TestBrowsingIsAdvertisedWithItsChannel(t *testing.T) {
	plain := RemoteControl(3, "Lanovo", 0x0017, 0, UUIDAVRemoteControlController, CategoryPlayer)

	if f, _ := plain.Attribute(AttrFeatures); f.Type != TypeNil {
		if v, _ := f.Uint(); v&FeatureBrowsing != 0 {
			t.Error("a record with no browsing channel still claims browsing")
		}
	}
	if _, ok := plain.Attribute(AttrMoreProtocols); ok {
		t.Error("a record with no browsing channel published a second protocol list")
	}

	browsing := RemoteControl(3, "Lanovo", 0x0017, 0x001b, UUIDAVRemoteControlController, CategoryPlayer)

	f, ok := browsing.Attribute(AttrFeatures)
	if !ok {
		t.Fatal("no supported features at all")
	}
	if v, _ := f.Uint(); v&FeatureBrowsing == 0 {
		t.Errorf("features %#04x does not claim browsing", v)
	}

	more, ok := browsing.Attribute(AttrMoreProtocols)
	if !ok {
		t.Fatal("browsing is claimed and no channel is published for it")
	}

	// A list of lists: the extra sequence is what keeps a phone from reading the browsing channel
	// as a second protocol layered on the control channel.
	if len(more.Children) != 1 {
		t.Fatalf("%d protocol descriptor lists, want one", len(more.Children))
	}

	psm, ok := PSM(more.Children[0])
	if !ok {
		t.Fatal("the published list carries no l2cap channel")
	}
	if psm != 0x001b {
		t.Errorf("browsing published on %#04x, want 0x001b", psm)
	}
}

// A search for two classes wants something that is both, and answering with something that is only
// one is how a device shows up and then cannot be used.
func TestWantsNeedsEveryUUIDInThePattern(t *testing.T) {
	record := AudioSink(1, "Lanovo", 0x0019, FeatureSpeaker)

	both := Search{Pattern: []uint32{UUIDAudioSink, UUIDA2DP}}
	if !both.Wants(record.UUIDs()) {
		t.Error("a sink searched for by both its class and its profile did not match")
	}

	missing := Search{Pattern: []uint32{UUIDAudioSink, UUIDAVRemoteControlTarget}}
	if missing.Wants(record.UUIDs()) {
		t.Error("the sink matched a pattern naming a class it does not have")
	}
}
