package avdtp

import "testing"

const ourSEID = 1

// config is what a phone sends to set up a stream: media transport, and one SBC configuration.
func config(sbc SBC) []byte {
	caps := []Capability{{Category: CatMediaTransport}, sbc.Capability()}

	out := []byte{AddressTo(ourSEID), AddressTo(2)}
	return append(out, MarshalCapabilities(caps)...)
}

// picked is one configuration out of what the sink offers, which is what a phone sends back.
func picked() SBC {
	return SBC{
		Rates:      Rate48000,
		Channels:   ChannelJointStereo,
		Blocks:     Block16,
		Subbands:   Subbands8,
		Allocation: AllocationLoudness,
		MinBitpool: 2,
		MaxBitpool: 53,
	}
}

func command(signal byte, data ...byte) Message {
	return Message{Label: 3, Type: Command, Signal: signal, Data: data}
}

// The whole conversation, the way a phone has it: find the endpoint, ask what it can do, configure
// it, open it, start it. Six messages and audio is flowing.
func TestAPhoneSetsUpAStream(t *testing.T) {
	var started, stopped int

	s := NewServer(ourSEID)
	s.Started = func(*Endpoint) { started++ }
	s.Stopped = func(*Endpoint) { stopped++ }

	// Discover.
	got := s.Handle(command(SignalDiscover))
	if got.Type != ResponseAccept {
		t.Fatalf("discover was refused: %x", got.Data)
	}
	seps, err := ParseSEPs(got.Data)
	if err != nil {
		t.Fatalf("ParseSEPs: %v", err)
	}
	if len(seps) != 1 || seps[0].SEID != ourSEID {
		t.Fatalf("discover answered %+v", seps)
	}
	if seps[0].TSEP != Sink || seps[0].Media != MediaAudio {
		t.Errorf("the endpoint is not an audio sink: %+v", seps[0])
	}
	if seps[0].InUse {
		t.Error("an idle endpoint reported itself in use")
	}

	// Get capabilities.
	got = s.Handle(command(SignalGetCapabilities, AddressTo(ourSEID)))
	if got.Type != ResponseAccept {
		t.Fatalf("get capabilities was refused: %x", got.Data)
	}
	caps, err := ParseCapabilities(got.Data)
	if err != nil {
		t.Fatalf("ParseCapabilities: %v", err)
	}
	if _, ok := Find(caps, CatMediaCodec); !ok {
		t.Error("the capabilities carried no codec")
	}

	// Set configuration.
	got = s.Handle(command(SignalSetConfig, config(picked())...))
	if got.Type != ResponseAccept {
		t.Fatalf("set configuration was refused: %x", got.Data)
	}
	if e := s.Endpoint(ourSEID); e.State != Configured {
		t.Fatalf("the endpoint is %v after configuring", e.State)
	}
	if e := s.Endpoint(ourSEID); e.Remote != 2 {
		t.Errorf("the phone's endpoint came back %d, want 2", e.Remote)
	}

	// Open.
	got = s.Handle(command(SignalOpen, AddressTo(ourSEID)))
	if got.Type != ResponseAccept {
		t.Fatalf("open was refused: %x", got.Data)
	}
	if e := s.Endpoint(ourSEID); e.State != Opened {
		t.Fatalf("the endpoint is %v after opening", e.State)
	}
	if started != 0 {
		t.Error("audio was reported started before the phone said start")
	}

	// Start.
	got = s.Handle(command(SignalStart, AddressTo(ourSEID)))
	if got.Type != ResponseAccept {
		t.Fatalf("start was refused: %x", got.Data)
	}
	if e := s.Endpoint(ourSEID); e.State != Streaming {
		t.Fatalf("the endpoint is %v after starting", e.State)
	}
	if started != 1 {
		t.Errorf("audio started %d times, want once", started)
	}

	// And the codec that was agreed is readable, which is what a decoder needs.
	sbc, ok := s.Endpoint(ourSEID).Codec()
	if !ok {
		t.Fatal("no codec on a streaming endpoint")
	}
	if rate, _ := sbc.Rate(); rate != 48000 {
		t.Errorf("the agreed rate is %d", rate)
	}

	// Suspend puts it back to open, and stops the audio without tearing anything down.
	got = s.Handle(command(SignalSuspend, AddressTo(ourSEID)))
	if got.Type != ResponseAccept {
		t.Fatalf("suspend was refused: %x", got.Data)
	}
	if e := s.Endpoint(ourSEID); e.State != Opened {
		t.Errorf("the endpoint is %v after suspending", e.State)
	}
	if stopped != 1 {
		t.Errorf("audio stopped %d times, want once", stopped)
	}

	// Close takes it all the way back.
	got = s.Handle(command(SignalClose, AddressTo(ourSEID)))
	if got.Type != ResponseAccept {
		t.Fatalf("close was refused: %x", got.Data)
	}
	if e := s.Endpoint(ourSEID); e.State != Idle || e.Config != nil {
		t.Errorf("the endpoint is %v with config %v after closing", e.State, e.Config)
	}
}

// A command in the wrong state has its own refusal, and a stack that answers something else leaves
// the phone retrying a thing that will never work.
func TestACommandInTheWrongStateSaysSo(t *testing.T) {
	s := NewServer(ourSEID)

	for _, signal := range []byte{SignalOpen, SignalStart, SignalSuspend, SignalClose} {
		got := s.Handle(command(signal, AddressTo(ourSEID)))
		if got.Type != ResponseReject {
			t.Errorf("signal %#x on an idle endpoint was accepted", signal)
			continue
		}
		if len(got.Data) == 0 || got.Data[len(got.Data)-1] != ErrorBadState {
			t.Errorf("signal %#x refused with %x, want bad state", signal, got.Data)
		}
	}
}

// Start and Suspend name a list, so their refusal has to say which endpoint failed — the phone
// asked about several and "no" without a subject tells it nothing.
func TestARefusedStartNamesTheEndpoint(t *testing.T) {
	s := NewServer(ourSEID)

	got := s.Handle(command(SignalStart, AddressTo(9)))
	if got.Type != ResponseReject {
		t.Fatal("starting an endpoint that does not exist was accepted")
	}
	if len(got.Data) != 2 {
		t.Fatalf("the refusal carried %d bytes, want the endpoint and the reason", len(got.Data))
	}
	if got.Data[0]>>2 != 9 {
		t.Errorf("the refusal named endpoint %d, want 9", got.Data[0]>>2)
	}
	if got.Data[1] != ErrorBadSEID {
		t.Errorf("the reason came back %#x", got.Data[1])
	}
}

// A list that is half applied leaves the phone and this device disagreeing about which streams are
// running, so nothing moves until every endpoint in it has been checked.
func TestAListIsCheckedBeforeAnythingMoves(t *testing.T) {
	s := NewServer(ourSEID)

	s.Handle(command(SignalSetConfig, config(picked())...))
	s.Handle(command(SignalOpen, AddressTo(ourSEID)))

	// One good endpoint and one that does not exist.
	got := s.Handle(command(SignalStart, AddressTo(ourSEID), AddressTo(9)))
	if got.Type != ResponseReject {
		t.Fatal("a list with a bad endpoint was accepted")
	}
	if e := s.Endpoint(ourSEID); e.State != Opened {
		t.Errorf("the good endpoint moved to %v despite the refusal", e.State)
	}
}

// A configuration is checked against what was offered, and the refusal names the category so the
// phone can try again.
func TestAConfigurationThatWasNeverOffered(t *testing.T) {
	s := NewServer(ourSEID)

	never := picked()
	never.Rates = Rate16000 // not in what the sink offers

	got := s.Handle(command(SignalSetConfig, config(never)...))
	if got.Type != ResponseReject {
		t.Fatal("a rate that was never offered was accepted")
	}
	if len(got.Data) != 2 || got.Data[0] != CatMediaCodec {
		t.Errorf("the refusal named %x, want the codec category first", got.Data)
	}
	if got.Data[1] != ErrorUnsupportedConfig {
		t.Errorf("the reason came back %#x", got.Data[1])
	}
	if e := s.Endpoint(ourSEID); e.State != Idle {
		t.Errorf("a refused configuration left the endpoint %v", e.State)
	}
}

// An offer sent back as a configuration is not a choice, and guessing which option was meant is how
// a stream runs at the wrong speed.
func TestAConfigurationThatDidNotChoose(t *testing.T) {
	s := NewServer(ourSEID)

	got := s.Handle(command(SignalSetConfig, config(SinkSBC())...))
	if got.Type != ResponseReject {
		t.Fatal("an offer sent back as a configuration was accepted")
	}
	if got.Data[0] != CatMediaCodec || got.Data[1] != ErrorBadPayloadFormat {
		t.Errorf("the refusal came back %x", got.Data)
	}
}

// Media transport has to be there. Without it there is no agreement about how audio arrives.
func TestAConfigurationWithNoMediaTransport(t *testing.T) {
	s := NewServer(ourSEID)

	data := []byte{AddressTo(ourSEID), AddressTo(2)}
	data = append(data, MarshalCapabilities([]Capability{picked().Capability()})...)

	got := s.Handle(command(SignalSetConfig, data...))
	if got.Type != ResponseReject {
		t.Fatal("a configuration with no media transport was accepted")
	}
	if got.Data[0] != CatMediaTransport {
		t.Errorf("the refusal named %#x, want media transport", got.Data[0])
	}
}

// Something we do not implement is refused by naming it, which is what lets the phone retry without
// it rather than give up.
func TestACapabilityWeDoNotImplement(t *testing.T) {
	s := NewServer(ourSEID)

	caps := []Capability{
		{Category: CatMediaTransport},
		picked().Capability(),
		{Category: CatContentProtect, Data: []byte{0x02, 0x00}},
	}
	data := append([]byte{AddressTo(ourSEID), AddressTo(2)}, MarshalCapabilities(caps)...)

	got := s.Handle(command(SignalSetConfig, data...))
	if got.Type != ResponseReject {
		t.Fatal("content protection was accepted")
	}
	if got.Data[0] != CatContentProtect || got.Data[1] != ErrorBadCategory {
		t.Errorf("the refusal came back %x", got.Data)
	}
}

// Configuring an endpoint somebody is already using is its own refusal, not a bad state.
func TestConfiguringAnEndpointTwice(t *testing.T) {
	s := NewServer(ourSEID)

	s.Handle(command(SignalSetConfig, config(picked())...))

	got := s.Handle(command(SignalSetConfig, config(picked())...))
	if got.Type != ResponseReject {
		t.Fatal("configuring an endpoint twice was accepted")
	}
	if got.Data[len(got.Data)-1] != ErrorSEPInUse {
		t.Errorf("the refusal came back %x, want in use", got.Data)
	}
}

// A discovered endpoint says whether it is busy, which is how a phone knows not to try.
func TestADiscoveredEndpointSaysWhetherItIsBusy(t *testing.T) {
	s := NewServer(ourSEID)
	s.Handle(command(SignalSetConfig, config(picked())...))

	seps, err := ParseSEPs(s.Handle(command(SignalDiscover)).Data)
	if err != nil {
		t.Fatalf("ParseSEPs: %v", err)
	}
	if !seps[0].InUse {
		t.Error("a configured endpoint did not report itself in use")
	}
}

// Abort is never refused for state. A phone aborts when it has lost track, and an abort that can
// fail leaves both ends stuck.
func TestAbortAlwaysWorks(t *testing.T) {
	for _, setup := range []func(*Server){
		func(*Server) {},
		func(s *Server) { s.Handle(command(SignalSetConfig, config(picked())...)) },
		func(s *Server) {
			s.Handle(command(SignalSetConfig, config(picked())...))
			s.Handle(command(SignalOpen, AddressTo(ourSEID)))
			s.Handle(command(SignalStart, AddressTo(ourSEID)))
		},
	} {
		s := NewServer(ourSEID)
		setup(s)

		got := s.Handle(command(SignalAbort, AddressTo(ourSEID)))
		if got.Type != ResponseAccept {
			t.Errorf("abort was refused: %x", got.Data)
		}
		if e := s.Endpoint(ourSEID); e.State != Idle {
			t.Errorf("the endpoint is %v after aborting", e.State)
		}
	}
}

// Aborting a stream that was running has to say the audio stopped, or whatever is decoding keeps
// waiting for bytes that will not come.
func TestAbortingAStreamStopsTheAudio(t *testing.T) {
	var stopped int

	s := NewServer(ourSEID)
	s.Stopped = func(*Endpoint) { stopped++ }

	s.Handle(command(SignalSetConfig, config(picked())...))
	s.Handle(command(SignalOpen, AddressTo(ourSEID)))
	s.Handle(command(SignalStart, AddressTo(ourSEID)))
	s.Handle(command(SignalAbort, AddressTo(ourSEID)))

	if stopped != 1 {
		t.Errorf("audio stopped %d times, want once", stopped)
	}
}

// A link dropping takes the stream with it, and has to say so for the same reason.
func TestForgettingALinkStopsTheAudio(t *testing.T) {
	var stopped int

	s := NewServer(ourSEID)
	s.Stopped = func(*Endpoint) { stopped++ }

	s.Handle(command(SignalSetConfig, config(picked())...))
	s.Handle(command(SignalOpen, AddressTo(ourSEID)))
	s.Handle(command(SignalStart, AddressTo(ourSEID)))

	s.Forget()

	if stopped != 1 {
		t.Errorf("audio stopped %d times, want once", stopped)
	}
	if e := s.Endpoint(ourSEID); e.State != Idle {
		t.Errorf("the endpoint is %v after the link went away", e.State)
	}
}

func TestAnEndpointThatDoesNotExist(t *testing.T) {
	s := NewServer(ourSEID)

	for _, signal := range []byte{SignalGetCapabilities, SignalGetConfig, SignalOpen, SignalClose} {
		got := s.Handle(command(signal, AddressTo(9)))
		if got.Type != ResponseReject {
			t.Errorf("signal %#x on a missing endpoint was accepted", signal)
			continue
		}
		if got.Data[len(got.Data)-1] != ErrorBadSEID {
			t.Errorf("signal %#x refused with %x, want a bad endpoint", signal, got.Data)
		}
	}
}

// A signal nobody implements gets a general reject, which is a different thing from a refusal.
func TestAnUnknownSignal(t *testing.T) {
	got := NewServer(ourSEID).Handle(command(0x3f))

	if got.Type != GeneralReject {
		t.Errorf("an unknown signal was answered with type %d, want a general reject", got.Type)
	}
}

// Nothing here sends commands, so a response arriving is one to something we never asked.
func TestAResponseIsNotAnswered(t *testing.T) {
	got := NewServer(ourSEID).Handle(Message{Label: 1, Type: ResponseAccept, Signal: SignalStart})

	if got.Signal != 0 || got.Type != 0 {
		t.Errorf("a response was answered with %+v", got)
	}
}

// Reading back what was agreed, which a phone does to check the two ends match.
func TestGettingTheConfigurationBack(t *testing.T) {
	s := NewServer(ourSEID)

	if got := s.Handle(command(SignalGetConfig, AddressTo(ourSEID))); got.Type != ResponseReject {
		t.Error("an idle endpoint answered with a configuration")
	}

	s.Handle(command(SignalSetConfig, config(picked())...))

	got := s.Handle(command(SignalGetConfig, AddressTo(ourSEID)))
	if got.Type != ResponseAccept {
		t.Fatalf("get configuration was refused: %x", got.Data)
	}

	caps, err := ParseCapabilities(got.Data)
	if err != nil {
		t.Fatalf("ParseCapabilities: %v", err)
	}
	codec, ok := Find(caps, CatMediaCodec)
	if !ok {
		t.Fatal("the configuration came back with no codec")
	}
	sbc, err := ParseSBC(codec)
	if err != nil {
		t.Fatalf("ParseSBC: %v", err)
	}
	if sbc != picked() {
		t.Errorf("the configuration came back %+v, want %+v", sbc, picked())
	}
}
