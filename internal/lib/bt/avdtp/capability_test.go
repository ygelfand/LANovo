package avdtp

import "testing"

func TestCapabilitiesGoOutAndComeBack(t *testing.T) {
	want := SinkCapabilities()

	got, err := ParseCapabilities(MarshalCapabilities(want))
	if err != nil {
		t.Fatalf("ParseCapabilities: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("%d capabilities came back, want %d", len(got), len(want))
	}

	// Media transport is empty and still has to be there: without it a phone has nothing to send
	// audio over, whatever the codec says.
	if _, ok := Find(got, CatMediaTransport); !ok {
		t.Error("the media transport capability was lost")
	}
	if _, ok := Find(got, CatMediaCodec); !ok {
		t.Error("the codec capability was lost")
	}
}

func TestCapabilitiesThatDoNotAddUp(t *testing.T) {
	for _, tc := range []struct {
		name string
		buf  []byte
	}{
		{"half a header", []byte{CatMediaCodec}},
		{"says more than it carries", []byte{CatMediaCodec, 0x06, 0x00, 0x00}},
	} {
		if _, err := ParseCapabilities(tc.buf); err == nil {
			t.Errorf("%s: was accepted", tc.name)
		}
	}
}

func TestTheSBCOfferGoesOutAndComesBack(t *testing.T) {
	want := SinkSBC()

	got, err := ParseSBC(want.Capability())
	if err != nil {
		t.Fatalf("ParseSBC: %v", err)
	}
	if got != want {
		t.Errorf("came back %+v, want %+v", got, want)
	}
}

// The two packed bytes split into five fields, and getting a mask wrong silently moves an option
// into the wrong one.
func TestTheSBCFieldsUnpackWhereTheyShould(t *testing.T) {
	s := SBC{
		Rates:      Rate48000,
		Channels:   ChannelJointStereo,
		Blocks:     Block16,
		Subbands:   Subbands8,
		Allocation: AllocationLoudness,
		MinBitpool: 2,
		MaxBitpool: 53,
	}

	c := s.Capability()
	if c.Data[0] != MediaAudio<<4 {
		t.Errorf("the media type byte is %#x", c.Data[0])
	}
	if c.Data[1] != CodecSBC {
		t.Errorf("the codec byte is %#x", c.Data[1])
	}

	got, err := ParseSBC(c)
	if err != nil {
		t.Fatalf("ParseSBC: %v", err)
	}
	if got != s {
		t.Errorf("came back %+v, want %+v", got, s)
	}
}

func TestWhatIsNotAnSBCCapability(t *testing.T) {
	if _, err := ParseSBC(Capability{Category: CatMediaTransport}); err == nil {
		t.Error("media transport was read as a codec")
	}
	if _, err := ParseSBC(Capability{Category: CatMediaCodec, Data: []byte{0, 0}}); err == nil {
		t.Error("a codec capability with no codec bytes was accepted")
	}

	// A codec that is not SBC. Nothing here decodes AAC or aptX and pretending otherwise would
	// configure a stream this device cannot play.
	aac := Capability{Category: CatMediaCodec, Data: []byte{MediaAudio << 4, 0x02, 0, 0, 0, 0}}
	if _, err := ParseSBC(aac); err == nil {
		t.Error("an AAC capability was read as SBC")
	}
}

// An offer has several bits set per field; a configuration has exactly one. Acting on an offer as
// though it were a choice is how a stream runs at the wrong rate.
func TestAnOfferIsNotAChoice(t *testing.T) {
	if SinkSBC().Chosen() {
		t.Error("the sink's offer reported itself as a configuration")
	}

	picked := SBC{
		Rates:      Rate48000,
		Channels:   ChannelJointStereo,
		Blocks:     Block16,
		Subbands:   Subbands8,
		Allocation: AllocationLoudness,
		MinBitpool: 2,
		MaxBitpool: 53,
	}
	if !picked.Chosen() {
		t.Error("a configuration with one bit per field did not report itself as one")
	}

	// Two rates is not a choice, and neither is none.
	two := picked
	two.Rates = Rate44100 | Rate48000
	if two.Chosen() {
		t.Error("two rates reported as a choice")
	}

	none := picked
	none.Channels = 0
	if none.Chosen() {
		t.Error("no channel mode reported as a choice")
	}
}

// The refusal every stack sends is for a configuration naming something never offered, so this is
// the check before accepting one.
func TestAConfigurationHasToBeWithinWhatWasOffered(t *testing.T) {
	offer := SinkSBC()

	good := SBC{
		Rates:      Rate48000,
		Channels:   ChannelJointStereo,
		Blocks:     Block16,
		Subbands:   Subbands8,
		Allocation: AllocationLoudness,
		MinBitpool: 2,
		MaxBitpool: 53,
	}
	if !good.Within(offer) {
		t.Error("a configuration picked from the offer was rejected")
	}

	// A rate never offered.
	fast := good
	fast.Rates = Rate16000
	if fast.Within(offer) {
		t.Error("a rate that was never offered was accepted")
	}

	// A bitpool above the ceiling, which is what makes a stream exceed the bandwidth agreed.
	greedy := good
	greedy.MaxBitpool = 250
	if greedy.Within(offer) {
		t.Error("a bitpool above the offered ceiling was accepted")
	}

	// And below the floor.
	thin := good
	thin.MinBitpool = 1
	if thin.Within(offer) {
		t.Error("a bitpool below the offered floor was accepted")
	}
}

func TestReadingTheRateAConfigurationPicked(t *testing.T) {
	for _, tc := range []struct {
		bits byte
		want int
	}{
		{Rate16000, 16000},
		{Rate32000, 32000},
		{Rate44100, 44100},
		{Rate48000, 48000},
	} {
		got, ok := (SBC{Rates: tc.bits}).Rate()
		if !ok || got != tc.want {
			t.Errorf("%#x came back %d (ok=%v), want %d", tc.bits, got, ok, tc.want)
		}
	}

	// Several bits set is an offer rather than a choice, and has no one rate to report.
	if _, ok := (SBC{Rates: Rate44100 | Rate48000}).Rate(); ok {
		t.Error("an offer of two rates reported a single one")
	}
}

// What this device offers has to include what phones actually ask for, or nothing connects.
func TestTheSinkOffersWhatAPhoneWillWant(t *testing.T) {
	s := SinkSBC()

	if s.Rates&Rate44100 == 0 {
		t.Error("44100 is not offered, and it is what most phones send")
	}
	if s.Rates&Rate48000 == 0 {
		t.Error("48000 is not offered")
	}
	if s.Channels&ChannelJointStereo == 0 {
		t.Error("joint stereo is not offered, and it is what a phone picks when it can")
	}
	if s.MaxBitpool < 53 {
		t.Errorf("the bitpool ceiling is %d, below the 53 that high quality sbc wants", s.MaxBitpool)
	}
}
