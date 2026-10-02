package l2cap

import "testing"

// opening is the request a manager makes for a protocol, and the channel number it chose.
func opening(t *testing.T, m *Manager, psm uint16) (Command, uint16) {
	t.Helper()

	c, err := m.Connect(psm)
	if err != nil {
		t.Fatalf("Connect(%#04x): %v", psm, err)
	}

	req, err := ParseConnect(c)
	if err != nil {
		t.Fatalf("the request did not parse: %v", err)
	}
	if req.PSM != psm {
		t.Fatalf("asked for %#04x, want %#04x", req.PSM, psm)
	}
	return c, req.SourceCID
}

// The whole of opening a channel from this end: ask, be accepted, both ends configure, open.
func TestOpeningAChannelOutwards(t *testing.T) {
	m := NewManager(895)

	req, ours := opening(t, m, PSMAVCTPBrowse)

	if got := m.Channel(ours); got == nil || got.State != Connecting {
		t.Fatalf("the channel is %v before an answer, want connecting", got)
	}

	// They accept, naming their own number for it.
	out := answer(t, m, Connected{
		DestinationCID: 0x0071,
		SourceCID:      ours,
		Result:         ConnectSuccess,
	}.Command(req.ID))

	// Our own configuration follows the acceptance without being asked for.
	asked := only(t, out)

	cfg, err := ParseConfigure(asked)
	if err != nil {
		t.Fatalf("what followed the acceptance was not a configuration: %v", err)
	}
	if cfg.DestinationCID != 0x0071 {
		t.Errorf("configured channel %#x, want theirs at %#x", cfg.DestinationCID, 0x0071)
	}

	ch := m.Channel(ours)
	if ch.State != Configuring {
		t.Fatalf("the channel is %v after an acceptance, want configuring", ch.State)
	}
	if ch.Remote != 0x0071 {
		t.Errorf("their number came out as %#x, want %#x", ch.Remote, 0x0071)
	}

	// They configure us, and we accept.
	answer(t, m, Configure{
		DestinationCID: ours,
		Options:        []Option{MTUOption(672)},
	}.Command(9))

	if ch.State != Configuring {
		t.Fatalf("one side configured and the channel is already %v", ch.State)
	}

	// They accept ours, answering on the identifier it was sent with. That is the last of the four
	// and opens the channel.
	answer(t, m, Configured{SourceCID: ours, Result: ConfigSuccess}.Command(asked.ID))

	if ch.State != Open {
		t.Errorf("both ends configured and the channel is %v, want open", ch.State)
	}
}

// A refusal takes the channel away rather than leaving it half made.
func TestARefusedChannelIsDropped(t *testing.T) {
	m := NewManager(895)
	req, ours := opening(t, m, PSMAVCTPBrowse)

	_, err := m.Handle(Signal(Connected{
		SourceCID: ours,
		Result:    ConnectBadPSM,
	}.Command(req.ID)))

	if err == nil {
		t.Error("a refused channel was not reported")
	}
	if got := m.Channel(ours); got != nil {
		t.Errorf("the channel is still here as %v after a refusal", got.State)
	}
}

// Pending is the far end asking for time, not answering. The request still stands.
func TestPendingLeavesTheRequestOutstanding(t *testing.T) {
	m := NewManager(895)
	req, ours := opening(t, m, PSMAVCTPBrowse)

	if out := answer(t, m, Connected{
		SourceCID: ours,
		Result:    ConnectPending,
	}.Command(req.ID)); len(out) != 0 {
		t.Errorf("pending drew %d commands, want none", len(out))
	}

	if got := m.Channel(ours); got == nil || got.State != Connecting {
		t.Fatalf("pending left the channel %v, want still connecting", got)
	}

	// And the acceptance that follows on the same identifier still lands.
	answer(t, m, Connected{
		DestinationCID: 0x0071,
		SourceCID:      ours,
		Result:         ConnectSuccess,
	}.Command(req.ID))

	if got := m.Channel(ours); got.State != Configuring {
		t.Errorf("the acceptance after a pending left the channel %v", got.State)
	}
}

// Asking twice for the same protocol is a caller that has lost track, not a second channel.
func TestOpeningTheSameProtocolTwice(t *testing.T) {
	m := NewManager(895)
	opening(t, m, PSMAVCTPBrowse)

	if _, err := m.Connect(PSMAVCTPBrowse); err == nil {
		t.Error("a second channel to the same protocol was allowed")
	}
}

// An answer to a request nobody made is ignored rather than acted on: a stale response must not
// open a channel or disturb one that is live.
func TestAnAnswerToNothingIsIgnored(t *testing.T) {
	m := NewManager(895)

	out := answer(t, m, Connected{
		DestinationCID: 0x0071,
		SourceCID:      0x0040,
		Result:         ConnectSuccess,
	}.Command(7))

	if len(out) != 0 {
		t.Errorf("%d commands answered a response nobody asked for", len(out))
	}
	if got := m.Channel(0x0040); got != nil {
		t.Error("a channel appeared from an unasked-for answer")
	}
}

// Numbers are each end's own. An acceptance naming a channel we did not ask about is refused rather
// than trusted, because acting on it would point our channel at somebody else's.
func TestAnAcceptanceForTheWrongChannel(t *testing.T) {
	m := NewManager(895)
	req, ours := opening(t, m, PSMAVCTPBrowse)

	_, err := m.Handle(Signal(Connected{
		DestinationCID: 0x0071,
		SourceCID:      ours + 1,
		Result:         ConnectSuccess,
	}.Command(req.ID)))

	if err == nil {
		t.Error("an acceptance for another channel was accepted")
	}
}

// A mode this stack cannot speak is answered with the one it can: unacceptable parameters, naming
// basic. An unknown-option answer leaves a peer that requires the mode with nothing to agree to.
func TestAnUnspeakableModeIsAnsweredWithOneWeSpeak(t *testing.T) {
	m := NewManager(895)
	req, ours := opening(t, m, PSMAVCTPBrowse)

	asked := only(t, answer(t, m, Connected{
		DestinationCID: 0x0071,
		SourceCID:      ours,
		Result:         ConnectSuccess,
	}.Command(req.ID)))

	// They configure us, asking for retransmission as the profile tells them to.
	out := answer(t, m, Configure{
		DestinationCID: ours,
		Options: []Option{
			MTUOption(672),
			{Type: OptionRetrans, Value: []byte{0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		},
	}.Command(11))

	got, err := ParseConfigured(only(t, out))
	if err != nil {
		t.Fatalf("ParseConfigured: %v", err)
	}
	if got.Result != ConfigUnacceptable {
		t.Fatalf("answered %#04x, want unacceptable parameters", got.Result)
	}

	var proposed []byte
	for _, o := range got.Options {
		if o.Type == OptionRetrans {
			proposed = o.Value
		}
	}
	if proposed == nil {
		t.Fatal("no mode was proposed back, so a peer has nothing to agree to")
	}
	if proposed[0] != ModeBasic {
		t.Errorf("proposed mode %#02x, want basic", proposed[0])
	}
	if len(proposed) != retransFields {
		t.Errorf("the option is %d bytes, want its fixed %d", len(proposed), retransFields)
	}

	if ch := m.Channel(ours); ch.State != Configuring {
		t.Fatalf("a refused option left the channel %v", ch.State)
	}

	// They come back without it, which is what the refusal invites.
	answer(t, m, Configure{
		DestinationCID: ours,
		Options:        []Option{MTUOption(672)},
	}.Command(12))

	answer(t, m, Configured{SourceCID: ours, Result: ConfigSuccess}.Command(asked.ID))

	if ch := m.Channel(ours); ch.State != Open {
		t.Errorf("after the retry the channel is %v, want open", ch.State)
	}
}

// Opening outwards and answering inwards share the numbering, so neither may hand out one the
// other is using.
func TestBothDirectionsShareTheNumbering(t *testing.T) {
	m := NewManager(895)
	m.Serve(PSMAVDTP)

	_, ours := opening(t, m, PSMAVCTPBrowse)

	out := answer(t, m, Connect{PSM: PSMAVDTP, SourceCID: 0x0041}.Command(3))
	got, err := ParseConnected(out[0])
	if err != nil {
		t.Fatalf("ParseConnected: %v", err)
	}

	if got.DestinationCID == ours {
		t.Errorf("an inbound channel was given %#x, which an outbound one already has", ours)
	}
}
