package bt

import (
	"encoding/binary"
	"testing"

	"github.com/ygelfand/LANovo/internal/lib/bt/avdtp"
	"github.com/ygelfand/LANovo/internal/lib/bt/avrcp"
	"github.com/ygelfand/LANovo/internal/lib/bt/l2cap"
	"github.com/ygelfand/LANovo/internal/lib/bt/sbc"
	"github.com/ygelfand/LANovo/internal/lib/bt/sdp"
)

// The layers are each tested on their own. What is tested here is that they compose: a phone doing
// what a phone does, from "what is this device" to samples, driven through the real Sink.
//
// Everything crosses the wire as bytes and is parsed back, so the framing is exercised rather than
// stepped around. A test that passed structs between the layers would not notice a length field
// written in the wrong endianness, which is the class of mistake this is for.

// phone is the far end, scripted.
type phone struct {
	t    *testing.T
	sink *Sink

	id    byte   // the next signalling command identifier
	cid   uint16 // the next channel number this side hands out
	label byte   // the next avdtp transaction label
}

func newPhone(t *testing.T) *phone {
	t.Helper()
	return &phone{t: t, sink: NewSink("Lanovo", l2cap.MTUDefault), cid: 0x0041}
}

func (p *phone) nextID() byte {
	p.id++
	return p.id
}

// wire sends one frame and returns the answers, both directions through Marshal and Parse.
func (p *phone) wire(f l2cap.Frame) []l2cap.Frame {
	p.t.Helper()

	sent, err := l2cap.ParseFrame(f.Marshal())
	if err != nil {
		p.t.Fatalf("our own frame did not parse: %v", err)
	}

	out, err := p.sink.Receive(sent)
	if err != nil {
		p.t.Fatalf("the sink refused a frame on channel %#x: %v", f.CID, err)
	}

	for i, r := range out {
		back, err := l2cap.ParseFrame(r.Marshal())
		if err != nil {
			p.t.Fatalf("the sink's answer did not parse: %v", err)
		}
		out[i] = back
	}
	return out
}

// commands sends a signalling frame and returns the commands that came back.
func (p *phone) commands(c ...l2cap.Command) []l2cap.Command {
	p.t.Helper()

	frames := p.wire(l2cap.Signal(c...))
	if len(frames) == 0 {
		return nil
	}
	if len(frames) != 1 {
		p.t.Fatalf("%d frames of signalling came back, want one", len(frames))
	}

	out, err := l2cap.ParseCommands(frames[0].Payload)
	if err != nil {
		p.t.Fatalf("the sink's signalling did not parse: %v", err)
	}
	return out
}

// channel is one open channel, from this side.
type channel struct {
	theirs uint16 // what the sink calls it, and what we address
	ours   uint16 // what we call it, and what the sink addresses
}

// open runs the whole four-way opening: connect, their configuration, ours.
func (p *phone) open(psm uint16) channel {
	p.t.Helper()

	ch := channel{ours: p.cid}
	p.cid++

	answers := p.commands(l2cap.Connect{PSM: psm, SourceCID: ch.ours}.Command(p.nextID()))
	if len(answers) != 2 {
		p.t.Fatalf("connecting to %#x gave %d commands, want an answer and a configuration",
			psm, len(answers))
	}

	connected, err := l2cap.ParseConnected(answers[0])
	if err != nil {
		p.t.Fatalf("ParseConnected: %v", err)
	}
	if connected.Result != l2cap.ConnectSuccess {
		p.t.Fatalf("connecting to %#x was refused: %#x", psm, connected.Result)
	}
	ch.theirs = connected.DestinationCID

	// Their configuration, which we accept as sent.
	if answers[1].Code != l2cap.CodeConfigRequest {
		p.t.Fatalf("the second command was %#x, want a configuration request", answers[1].Code)
	}
	// Named by the sink's own number. A response tells the other end which of its requests this
	// answers, so it carries their number and not the responder's.
	p.commands(l2cap.Configured{
		SourceCID: ch.theirs,
		Result:    l2cap.ConfigSuccess,
	}.Command(answers[1].ID))

	// Then ours, which they answer.
	answers = p.commands(l2cap.Configure{
		DestinationCID: ch.theirs,
		Options:        []l2cap.Option{l2cap.MTUOption(l2cap.MTUDefault)},
	}.Command(p.nextID()))

	if len(answers) != 1 {
		p.t.Fatalf("our configuration got %d answers, want one", len(answers))
	}
	configured, err := l2cap.ParseConfigured(answers[0])
	if err != nil {
		p.t.Fatalf("ParseConfigured: %v", err)
	}
	if configured.Result != l2cap.ConfigSuccess {
		p.t.Fatalf("our configuration was refused: %#x", configured.Result)
	}

	if state := p.sink.l2.Channel(ch.theirs).State; state != l2cap.Open {
		p.t.Fatalf("after configuring both ends the channel is %v", state)
	}
	return ch
}

// close tears a channel down the way a phone leaving does.
func (p *phone) close(ch channel) {
	p.t.Helper()

	p.commands(l2cap.Disconnect{
		DestinationCID: ch.theirs,
		SourceCID:      ch.ours,
	}.Command(l2cap.CodeDisconnectRequest, p.nextID()))
}

// data sends a payload on an open channel and returns the payloads that came back.
func (p *phone) data(ch channel, payload []byte) [][]byte {
	p.t.Helper()

	var out [][]byte
	for _, f := range p.wire(l2cap.Frame{CID: ch.theirs, Payload: payload}) {
		if f.CID != ch.ours {
			p.t.Fatalf("an answer came back on channel %#x, want %#x", f.CID, ch.ours)
		}
		out = append(out, f.Payload)
	}
	return out
}

// searchAttribute builds the request a phone actually sends: a pattern, a cap, a list of attribute
// ranges, and an empty continuation.
func (p *phone) searchAttribute(class uint16, attrs ...sdp.AttrRange) sdp.PDU {
	p.t.Helper()

	pattern, err := sdp.Sequence(sdp.UUID16(class)).Marshal()
	if err != nil {
		p.t.Fatalf("marshalling the pattern: %v", err)
	}

	elements := make([]sdp.Element, 0, len(attrs))
	for _, a := range attrs {
		elements = append(elements, a.Element())
	}
	list, err := sdp.Sequence(elements...).Marshal()
	if err != nil {
		p.t.Fatalf("marshalling the attribute list: %v", err)
	}

	params := append([]byte(nil), pattern...)
	params = binary.BigEndian.AppendUint16(params, 0xffff)
	params = append(params, list...)
	params = append(params, 0)

	return sdp.PDU{ID: sdp.PDUSearchAttributeRequest, Transaction: 1, Params: params}
}

// ask sends one AVDTP command and returns the answer.
func (p *phone) ask(ch channel, signal byte, data ...byte) avdtp.Message {
	p.t.Helper()

	m := avdtp.Message{Label: p.label, Type: avdtp.Command, Signal: signal, Data: data}
	p.label = (p.label + 1) & 0x0f

	payloads := p.data(ch, m.Marshal())
	if len(payloads) != 1 {
		p.t.Fatalf("signal %#x got %d answers, want one", signal, len(payloads))
	}

	answer, err := avdtp.ParseMessage(payloads[0])
	if err != nil {
		p.t.Fatalf("the sink's answer did not parse: %v", err)
	}
	if answer.Label != m.Label {
		p.t.Errorf("the answer to %#x came back on label %d, want %d",
			signal, answer.Label, m.Label)
	}
	if answer.Signal != signal {
		p.t.Errorf("asked %#x and was answered %#x", signal, answer.Signal)
	}
	return answer
}

// frame is a whole SBC frame with a correct check byte, so the sink will decode it.
func frame(t *testing.T, bitpool int, fill byte) []byte {
	t.Helper()

	// 44100 is index 2, sixteen blocks is index 3, eight subbands is index 1.
	raw := []byte{
		sbc.Syncword,
		2<<6 | 3<<4 | byte(sbc.JointStereo)<<2 | byte(sbc.Loudness)<<1 | 1,
		byte(bitpool),
		0,
	}

	h, err := sbc.ParseHeader(raw)
	if err != nil {
		t.Fatalf("our own header did not parse: %v", err)
	}

	out := make([]byte, h.Length())
	copy(out, raw)
	for i := 4; i < len(out); i++ {
		out[i] = fill
	}

	crc, err := sbc.CRC(h, out)
	if err != nil {
		t.Fatalf("CRC: %v", err)
	}
	out[3] = crc

	return out
}

// The whole thing: a phone finds the sink over SDP, negotiates a stream over AVDTP, and the audio
// it sends comes out as decodable frames.
func TestAPhoneFindsTheSinkAndPlaysToIt(t *testing.T) {
	p := newPhone(t)

	var started, stopped int
	var decoded [][]byte
	p.sink.Started = func(*avdtp.Endpoint) { started++ }
	p.sink.Stopped = func(*avdtp.Endpoint) { stopped++ }
	p.sink.Frame = func(f []byte) { decoded = append(decoded, f) }

	// Find out what this device is.
	discovery := p.open(l2cap.PSMSDP)

	request := p.searchAttribute(sdp.UUIDAudioSink, sdp.AttrRange{First: 0x0000, Last: 0xffff})
	answers := p.data(discovery, request.Marshal())
	if len(answers) != 1 {
		t.Fatalf("the search got %d answers, want one", len(answers))
	}

	response, err := sdp.ParsePDU(answers[0])
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}
	if response.ID != sdp.PDUSearchAttributeResponse {
		t.Fatalf("the search was answered with %#x", response.ID)
	}

	psm := psmFrom(t, response)
	if psm != SinkPSM {
		t.Fatalf("the record advertises channel %#x, the stack serves %#x", psm, SinkPSM)
	}

	// Connect to where the record said, and negotiate.
	signalling := p.open(psm)

	seps, err := avdtp.ParseSEPs(p.ask(signalling, avdtp.SignalDiscover).Data)
	if err != nil {
		t.Fatalf("ParseSEPs: %v", err)
	}
	if len(seps) != 1 {
		t.Fatalf("%d endpoints, want one", len(seps))
	}
	if seps[0].TSEP != avdtp.Sink || seps[0].Media != avdtp.MediaAudio {
		t.Fatalf("the endpoint is %+v, want an audio sink", seps[0])
	}
	seid := seps[0].SEID

	caps := p.ask(signalling, avdtp.SignalGetCapabilities, avdtp.AddressTo(seid))
	if caps.Type != avdtp.ResponseAccept {
		t.Fatalf("get capabilities was refused: %v", caps.Data)
	}

	offered, err := avdtp.ParseCapabilities(caps.Data)
	if err != nil {
		t.Fatalf("ParseCapabilities: %v", err)
	}
	codec, ok := avdtp.Find(offered, avdtp.CatMediaCodec)
	if !ok {
		t.Fatal("the sink offered no codec")
	}
	offer, err := avdtp.ParseSBC(codec)
	if err != nil {
		t.Fatalf("ParseSBC: %v", err)
	}

	// Pick one of everything, the way a phone does.
	chosen := avdtp.SBC{
		Rates:      avdtp.Rate48000,
		Channels:   avdtp.ChannelJointStereo,
		Blocks:     avdtp.Block16,
		Subbands:   avdtp.Subbands8,
		Allocation: avdtp.AllocationLoudness,
		MinBitpool: offer.MinBitpool,
		MaxBitpool: offer.MaxBitpool,
	}

	config := append([]byte{avdtp.AddressTo(seid), avdtp.AddressTo(2)},
		avdtp.MarshalCapabilities([]avdtp.Capability{
			{Category: avdtp.CatMediaTransport},
			chosen.Capability(),
		})...)

	if m := p.ask(signalling, avdtp.SignalSetConfig, config...); m.Type != avdtp.ResponseAccept {
		t.Fatalf("the configuration was refused: %v", m.Data)
	}
	if m := p.ask(signalling, avdtp.SignalOpen, avdtp.AddressTo(seid)); m.Type != avdtp.ResponseAccept {
		t.Fatalf("open was refused: %v", m.Data)
	}

	// The second channel to the same number is the audio. Which one it is is decided by order and
	// by nothing in the packets, so opening it is the part worth checking.
	transport := p.open(SinkPSM)
	if p.sink.transport != transport.theirs {
		t.Fatalf("the sink took channel %#x for media, want %#x",
			p.sink.transport, transport.theirs)
	}
	if p.sink.signalling != signalling.theirs {
		t.Fatalf("the sink took channel %#x for signalling, want %#x",
			p.sink.signalling, signalling.theirs)
	}

	if m := p.ask(signalling, avdtp.SignalStart, avdtp.AddressTo(seid)); m.Type != avdtp.ResponseAccept {
		t.Fatalf("start was refused: %v", m.Data)
	}
	if started != 1 {
		t.Fatalf("the stream started %d times", started)
	}

	// Two frames in one packet, which is what an encoder sends.
	first := frame(t, 35, 0x5a)
	second := frame(t, 35, 0xa5)

	media := avdtp.Media{
		Sequence: 1,
		Count:    2,
		Payload:  append(append([]byte(nil), first...), second...),
	}
	if out := p.data(transport, media.Marshal()); out != nil {
		t.Errorf("the sink answered a media packet with %d frames", len(out))
	}

	if len(decoded) != 2 {
		t.Fatalf("%d frames came out of one packet holding two", len(decoded))
	}

	// The frames have to come out whole and in order, or what reaches the decoder is noise.
	for i, want := range [][]byte{first, second} {
		if string(decoded[i]) != string(want) {
			t.Errorf("frame %d came out %d bytes, want %d", i, len(decoded[i]), len(want))
		}
	}

	// And they have to be decodable, which is the only check that the length arithmetic agreed all
	// the way from the packet down to the frame.
	for i, f := range decoded {
		if _, err := sbc.Unpack(f); err != nil {
			t.Errorf("frame %d did not decode: %v", i, err)
		}
	}

	if m := p.ask(signalling, avdtp.SignalSuspend, avdtp.AddressTo(seid)); m.Type != avdtp.ResponseAccept {
		t.Fatalf("suspend was refused: %v", m.Data)
	}
	if stopped != 1 {
		t.Errorf("the stream stopped %d times", stopped)
	}
}

// psmFrom digs the channel number out of a search response the way a phone does.
func psmFrom(t *testing.T, p sdp.PDU) uint16 {
	t.Helper()

	// A length, then the outer sequence of attribute lists, then the continuation.
	if len(p.Params) < 3 {
		t.Fatalf("the response is %d bytes", len(p.Params))
	}
	body := p.Params[2 : 2+int(binary.BigEndian.Uint16(p.Params))]

	outer, _, err := sdp.ParseElement(body)
	if err != nil {
		t.Fatalf("the response body: %v", err)
	}
	if len(outer.Children) != 1 {
		t.Fatalf("%d records came back, want one", len(outer.Children))
	}

	// Each record is a flat run of identifier and value.
	attrs := outer.Children[0].Children
	for i := 0; i+1 < len(attrs); i += 2 {
		id, ok := attrs[i].Uint()
		if !ok || uint16(id) != sdp.AttrProtocols {
			continue
		}
		psm, ok := sdp.PSM(attrs[i+1])
		if !ok {
			t.Fatal("the protocol list carries no channel number")
		}
		return psm
	}

	t.Fatal("the record came back with no protocol list")
	return 0
}

// The remote control is a third channel, to its own number, and unlike the AVDTP pair there is
// only ever one of it.
func TestThePhoneOpensTheRemoteControlAndPressesPlay(t *testing.T) {
	p := newPhone(t)

	var state avrcp.State
	p.sink.Remote().Changed = func(s avrcp.State) { state = s }

	if p.sink.Controlling() {
		t.Fatal("the remote control is up before anything opened it")
	}

	control := p.open(ControlPSM)
	if !p.sink.Controlling() {
		t.Fatal("the remote control channel did not come up")
	}
	if p.sink.control != control.theirs {
		t.Fatalf("the sink took channel %#x for control, want %#x",
			p.sink.control, control.theirs)
	}

	// A button on the screen goes out as two messages, on the control channel.
	frames, err := p.sink.Press(avrcp.OpPlay)
	if err != nil {
		t.Fatalf("Press: %v", err)
	}
	if len(frames) != 2 {
		t.Fatalf("%d frames, want a press and a release", len(frames))
	}

	for i, want := range []bool{avrcp.Pressed, avrcp.Released} {
		if frames[i].CID != control.ours {
			t.Errorf("frame %d went to channel %#x, want %#x", i, frames[i].CID, control.ours)
		}

		m, err := avrcp.ParseTransport(frames[i].Payload)
		if err != nil {
			t.Fatalf("frame %d did not parse: %v", i, err)
		}
		f, err := avrcp.ParseAVC(m.Payload)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}

		op, released, err := avrcp.Button(f)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if op != avrcp.OpPlay || released != want {
			t.Errorf("frame %d is %#02x released=%v", i, op, released)
		}
	}

	// The phone setting the volume, which is the half of the profile this device is the target for.
	command := avrcp.Transport{
		Label:   4,
		Type:    avrcp.MessageCommand,
		Payload: avrcp.SetAbsoluteVolume(avrcp.Volume(60)).Frame(avrcp.Control).Marshal(),
	}

	answers := p.data(control, command.Marshal())
	if len(answers) != 1 {
		t.Fatalf("%d answers to a volume command, want one", len(answers))
	}

	back, err := avrcp.ParseTransport(answers[0])
	if err != nil {
		t.Fatalf("the answer did not parse: %v", err)
	}
	if back.Type != avrcp.MessageResponse || back.Label != 4 {
		t.Errorf("answered as %+v", back)
	}
	if state.Volume < 59 || state.Volume > 61 {
		t.Errorf("the volume came through as %d%%, want about 60", state.Volume)
	}
}

// A search for the sink alone matches one record. The remote control ones are different classes
// and answering with them would be answering a question nobody asked.
func TestASearchForTheSinkMatchesOnlyTheSink(t *testing.T) {
	p := newPhone(t)
	discovery := p.open(l2cap.PSMSDP)

	request := p.searchAttribute(sdp.UUIDAudioSink, sdp.AttrRange{First: 0x0000, Last: 0xffff})
	answers := p.data(discovery, request.Marshal())

	response, err := sdp.ParsePDU(answers[0])
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}
	if got := len(recordsIn(t, response)); got != 1 {
		t.Errorf("%d records came back for the audio sink alone", got)
	}
}

// A phone that searches the public browse group is asking what this device offers, and all three
// have to come back in the one answer. Sending one makes the other two look absent.
func TestSearchingTheBrowseGroupFindsEverything(t *testing.T) {
	p := newPhone(t)
	discovery := p.open(l2cap.PSMSDP)

	request := p.searchAttribute(sdp.UUIDBrowseRoot, sdp.AttrRange{First: 0x0000, Last: 0xffff})
	answers := p.data(discovery, request.Marshal())

	response, err := sdp.ParsePDU(answers[0])
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}
	if got := len(recordsIn(t, response)); got != 3 {
		t.Errorf("%d records came back, want the sink and both ends of the remote control", got)
	}
}

// recordsIn is the attribute lists a search response carries.
func recordsIn(t *testing.T, p sdp.PDU) []sdp.Element {
	t.Helper()

	if len(p.Params) < 3 {
		t.Fatalf("the response is %d bytes", len(p.Params))
	}
	body := p.Params[2 : 2+int(binary.BigEndian.Uint16(p.Params))]

	outer, _, err := sdp.ParseElement(body)
	if err != nil {
		t.Fatalf("the response body: %v", err)
	}
	return outer.Children
}

// Both remote control records have to name the channel the stack actually serves, and it is not
// the audio one.
func TestTheRemoteControlRecordsNameTheControlChannel(t *testing.T) {
	records := Records("Lanovo")
	if len(records) != 3 {
		t.Fatalf("%d records", len(records))
	}

	for _, r := range records[1:] {
		protocols, ok := r.Attribute(sdp.AttrProtocols)
		if !ok {
			t.Fatal("a remote control record has no protocol list")
		}

		psm, ok := sdp.PSM(protocols)
		if !ok {
			t.Fatal("a remote control record carries no channel number")
		}
		if psm != ControlPSM {
			t.Errorf("a remote control record advertises %#x, want %#x", psm, ControlPSM)
		}
	}
}

// Both remote control records claim every category.
//
// A phone reading them looks up the generic remote control class and takes the features off the
// first record carrying it. Both records carry that class, so one claiming fewer categories than
// the other means what this device is judged capable of depends on which was written first — and
// a target record saying amplifier alone is what kept a phone on the single-player model.
//
// Only the categories. The bits above them mean different things at each end.
func TestTheRemoteControlRecordsAgreeOnTheirCategories(t *testing.T) {
	for _, r := range Records("Lanovo")[1:] {
		classes, ok := r.Attribute(sdp.AttrServiceClasses)
		if !ok {
			t.Fatal("a remote control record has no service classes")
		}

		// The class list itself, not the record. Every record mentions the generic class in its
		// profile descriptor, and that is not what a phone matches a class search against.
		generic := false
		for _, c := range classes.Children {
			if v, ok := c.Uint(); ok && v == sdp.UUIDAVRemoteControl {
				generic = true
			}
		}
		if !generic {
			t.Error("a remote control record does not carry the generic class")
		}

		features, ok := r.Attribute(sdp.AttrFeatures)
		if !ok {
			t.Fatal("a remote control record has no supported features")
		}

		v, ok := features.Uint()
		if !ok {
			t.Fatal("supported features is not a number")
		}

		if v&RemoteFeatures != RemoteFeatures {
			t.Errorf("a remote control record claims %#04x, missing a category from %#04x",
				v, uint32(RemoteFeatures))
		}
	}
}

// Every record needs its own handle: a phone uses one to ask about the same record again, and two
// records sharing a handle make one of them unreachable.
func TestEveryRecordHasItsOwnHandle(t *testing.T) {
	seen := map[uint32]bool{}

	for _, r := range Records("Lanovo") {
		handle, ok := r.Attribute(sdp.AttrRecordHandle)
		if !ok {
			t.Fatal("a record has no handle")
		}

		v, ok := handle.Uint()
		if !ok {
			t.Fatal("a handle is not a number")
		}
		if seen[v] {
			t.Errorf("handle %#x is used twice", v)
		}
		seen[v] = true
	}
}

// A record that names a channel the stack does not serve is a device a phone finds and cannot use,
// which looks like broken audio rather than a missing feature.
func TestTheRecordAdvertisesTheChannelTheStackServes(t *testing.T) {
	protocols, ok := Record("Lanovo").Attribute(sdp.AttrProtocols)
	if !ok {
		t.Fatal("the record has no protocol list")
	}

	psm, ok := sdp.PSM(protocols)
	if !ok {
		t.Fatal("the protocol list carries no channel number")
	}
	if psm != SinkPSM {
		t.Errorf("the record advertises %#x and the stack serves %#x", psm, SinkPSM)
	}
}

// A search for something this device is not gets an empty list rather than the record anyway.
func TestASearchForSomethingElseMatchesNothing(t *testing.T) {
	p := newPhone(t)
	discovery := p.open(l2cap.PSMSDP)

	// A headset, which this is not.
	request := p.searchAttribute(0x1108, sdp.AttrRange{First: 0x0000, Last: 0xffff})

	answers := p.data(discovery, request.Marshal())
	if len(answers) != 1 {
		t.Fatalf("%d answers", len(answers))
	}

	response, err := sdp.ParsePDU(answers[0])
	if err != nil {
		t.Fatalf("ParsePDU: %v", err)
	}
	if response.ID != sdp.PDUSearchAttributeResponse {
		t.Fatalf("answered with %#x", response.ID)
	}

	body := response.Params[2 : 2+int(binary.BigEndian.Uint16(response.Params))]
	outer, _, err := sdp.ParseElement(body)
	if err != nil {
		t.Fatalf("the response body: %v", err)
	}
	if len(outer.Children) != 0 {
		t.Errorf("%d records came back for a service this device does not offer",
			len(outer.Children))
	}
}

// Connecting to a channel nothing serves is refused with the reason that says so. A phone told no
// moves on; a phone told nothing waits.
func TestAnUnservedChannelIsRefused(t *testing.T) {
	p := newPhone(t)

	answers := p.commands(l2cap.Connect{PSM: 0x0003, SourceCID: 0x0041}.Command(p.nextID()))
	if len(answers) != 1 {
		t.Fatalf("%d commands came back, want one refusal", len(answers))
	}

	connected, err := l2cap.ParseConnected(answers[0])
	if err != nil {
		t.Fatalf("ParseConnected: %v", err)
	}
	if connected.Result != l2cap.ConnectBadPSM {
		t.Errorf("refused with %#x, want the one that names the channel", connected.Result)
	}
}

// Data on a channel that was never opened is an error rather than a panic, because it is what a
// stale packet after a disconnection looks like.
func TestDataOnAChannelThatIsNotOpen(t *testing.T) {
	p := newPhone(t)

	if _, err := p.sink.Receive(l2cap.Frame{CID: 0x0050, Payload: []byte{1, 2}}); err == nil {
		t.Error("data on an unopened channel was accepted")
	}
}

// The far end closes what it opened and nothing else, so the channels this end opened outlive the
// session unless they are dropped with it.
func TestASessionEndingDropsWhatThisEndOpened(t *testing.T) {
	p := newPhone(t)

	control := p.open(ControlPSM)
	browse := p.open(BrowsePSM)

	if p.sink.control == 0 || p.sink.browse == 0 {
		t.Fatalf("control %#x and browsing %#x, want both", p.sink.control, p.sink.browse)
	}

	p.close(control)

	if p.sink.control != 0 || p.sink.browse != 0 {
		t.Errorf("the session ended and control is %#x, browsing %#x",
			p.sink.control, p.sink.browse)
	}
	if p.sink.l2.Channel(browse.theirs) != nil {
		t.Error("the browsing channel outlived the session it belonged to")
	}
}

// A phone that comes back opens new channels. The stream follows those rather than the numbers the
// last one used, which is what a phone whose first message went nowhere was hitting.
func TestANewSessionTakesTheNewStreamChannels(t *testing.T) {
	p := newPhone(t)

	first := p.open(SinkPSM)
	if p.sink.signalling != first.theirs {
		t.Fatalf("signalling is %#x, want %#x", p.sink.signalling, first.theirs)
	}

	p.close(first)
	if p.sink.signalling != 0 {
		t.Errorf("a closed channel is still the stream: %#x", p.sink.signalling)
	}

	second := p.open(SinkPSM)
	if p.sink.signalling != second.theirs {
		t.Errorf("signalling is %#x, want the new channel %#x", p.sink.signalling, second.theirs)
	}
}

// A link that goes away leaves nothing behind, or the next phone to connect inherits the last
// one's stream and the channel numbers it was using.
func TestForgettingALink(t *testing.T) {
	p := newPhone(t)
	p.open(l2cap.PSMSDP)
	signalling := p.open(SinkPSM)

	p.ask(signalling, avdtp.SignalDiscover)
	p.sink.Forget()

	if got := p.sink.l2.Open(); len(got) != 0 {
		t.Errorf("%d channels survived", len(got))
	}
	if p.sink.signalling != 0 || p.sink.transport != 0 {
		t.Errorf("the avdtp channels survived: %#x and %#x", p.sink.signalling, p.sink.transport)
	}
	if e := p.sink.Endpoint(); e.State != avdtp.Idle {
		t.Errorf("the endpoint is %v, want idle", e.State)
	}
}
