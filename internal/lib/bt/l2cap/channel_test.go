package l2cap

import "testing"

// answer runs one command through a manager and gives back what it said.
func answer(t *testing.T, m *Manager, commands ...Command) []Command {
	t.Helper()

	out, err := m.Handle(Signal(commands...))
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	return out
}

// only is the single command expected back, failing if there is not exactly one.
func only(t *testing.T, out []Command) Command {
	t.Helper()

	if len(out) != 1 {
		t.Fatalf("%d commands came back, want one", len(out))
	}
	return out[0]
}

// The whole of opening a channel, the way a phone does it: connect, both ends configure, open.
func TestAPhoneOpensAChannel(t *testing.T) {
	m := NewManager(895)
	m.Serve(PSMAVDTP)

	// It asks for the audio protocol, naming the number it will know the channel by.
	out := answer(t, m, Connect{PSM: PSMAVDTP, SourceCID: 0x0041}.Command(1))

	// An acceptance, and our own configuration request alongside it.
	if len(out) != 2 {
		t.Fatalf("%d commands came back, want an acceptance and a configuration", len(out))
	}

	got, err := ParseConnected(out[0])
	if err != nil {
		t.Fatalf("ParseConnected: %v", err)
	}
	if got.Result != ConnectSuccess {
		t.Fatalf("the connection was refused: %#x", got.Result)
	}
	if got.SourceCID != 0x0041 {
		t.Errorf("the answer carried their channel as %#x", got.SourceCID)
	}
	if got.DestinationCID < CIDDynamic {
		t.Errorf("we took channel %#x, below the dynamic range", got.DestinationCID)
	}
	local := got.DestinationCID

	if out[1].Code != CodeConfigRequest {
		t.Fatalf("the second command was %#x, want a configuration request", out[1].Code)
	}
	ours, err := ParseConfigure(out[1])
	if err != nil {
		t.Fatalf("ParseConfigure: %v", err)
	}
	if ours.DestinationCID != 0x0041 {
		t.Errorf("we configured %#x, want their channel", ours.DestinationCID)
	}
	if ours.MTU() != 895 {
		t.Errorf("we asked for an mtu of %d, want 895", ours.MTU())
	}

	// Not open yet: only one end has configured.
	if ch := m.Channel(local); ch == nil || ch.State != Configuring {
		t.Fatalf("the channel is %v after one side configured", ch.State)
	}

	// They configure us.
	res := only(t, answer(t, m, Configure{
		DestinationCID: local,
		Options:        []Option{MTUOption(672)},
	}.Command(2)))

	if res.Code != CodeConfigResponse {
		t.Fatalf("their configuration was answered with %#x", res.Code)
	}
	if got, _ := ParseConfigured(res); got.Result != ConfigSuccess {
		t.Errorf("we refused their configuration: %#x", got.Result)
	}
	if res.ID != 2 {
		t.Errorf("the answer carried identifier %d, want the question's 2", res.ID)
	}

	// Still not open: they have not answered ours.
	if ch := m.Channel(local); ch.State != Configuring {
		t.Fatalf("the channel opened before both ends configured")
	}

	// They answer ours, naming the channel by our number rather than theirs — a response says
	// which of our requests it belongs to, and ours is the only number we could look that up by.
	answer(t, m, Configured{SourceCID: local, Result: ConfigSuccess}.Command(out[1].ID))

	ch := m.Channel(local)
	if ch.State != Open {
		t.Fatalf("the channel is %v after both ends configured, want open", ch.State)
	}
	if ch.MTU != 672 {
		t.Errorf("the channel carries an mtu of %d, want the 672 they asked for", ch.MTU)
	}
	if ch.PSM != PSMAVDTP {
		t.Errorf("the channel is for psm %#x", ch.PSM)
	}
	if len(m.Open()) != 1 {
		t.Errorf("%d channels report open", len(m.Open()))
	}
}

// A protocol nothing serves is refused with the reason that says so. Told no, a phone moves on;
// told nothing, it waits.
func TestAProtocolNobodyServesIsRefused(t *testing.T) {
	m := NewManager(672)
	m.Serve(PSMAVDTP)

	out := only(t, answer(t, m, Connect{PSM: PSMSDP, SourceCID: 0x0041}.Command(1)))

	got, err := ParseConnected(out)
	if err != nil {
		t.Fatalf("ParseConnected: %v", err)
	}
	if got.Result != ConnectBadPSM {
		t.Errorf("result %#x, want the psm refusal", got.Result)
	}
	if got.DestinationCID != 0 {
		t.Errorf("a refused channel was given number %#x", got.DestinationCID)
	}
	if len(m.Open()) != 0 {
		t.Error("a refused connection left a channel behind")
	}
}

// Two channels at once is the ordinary case — SDP to find the device, AVDTP to stream — and they
// must not be given the same number.
func TestTwoChannelsGetDifferentNumbers(t *testing.T) {
	m := NewManager(672)
	m.Serve(PSMSDP)
	m.Serve(PSMAVDTP)

	first, _ := ParseConnected(answer(t, m, Connect{PSM: PSMSDP, SourceCID: 0x0041}.Command(1))[0])

	second, _ := ParseConnected(answer(t, m, Connect{PSM: PSMAVDTP, SourceCID: 0x0042}.Command(2))[0])

	if first.DestinationCID == second.DestinationCID {
		t.Fatalf("both channels were given %#x", first.DestinationCID)
	}
	if m.Channel(first.DestinationCID).PSM != PSMSDP {
		t.Error("the first channel forgot what it was for")
	}
	if m.Channel(second.DestinationCID).PSM != PSMAVDTP {
		t.Error("the second channel forgot what it was for")
	}
}

// Configuration in two parts, which is what happens when the options do not fit one command. The
// flag has to come back or the far end waits for an acknowledgement it never gets.
func TestAConfigurationThatArrivesInTwoParts(t *testing.T) {
	m := NewManager(672)
	m.Serve(PSMAVDTP)

	opened, _ := ParseConnected(answer(t, m, Connect{PSM: PSMAVDTP, SourceCID: 0x0041}.Command(1))[0])
	local := opened.DestinationCID

	part := only(t, answer(t, m, Configure{
		DestinationCID: local,
		Flags:          ConfigFlagContinues,
		Options:        []Option{MTUOption(895)},
	}.Command(2)))

	got, _ := ParseConfigured(part)
	if got.Flags&ConfigFlagContinues == 0 {
		t.Error("the continuation flag was not carried back")
	}
	if m.Channel(local).theirs {
		t.Error("a part of a configuration was treated as the whole of it")
	}

	answer(t, m, Configure{DestinationCID: local}.Command(3))
	if !m.Channel(local).theirs {
		t.Error("the last part did not finish their side")
	}
}

// An option that has to be answered and cannot be is named back, which is what lets the far end try
// again without it.
func TestAnUnknownOptionIsNamedRatherThanIgnored(t *testing.T) {
	m := NewManager(672)
	m.Serve(PSMAVDTP)

	opened, _ := ParseConnected(answer(t, m, Connect{PSM: PSMAVDTP, SourceCID: 0x0041}.Command(1))[0])

	out := only(t, answer(t, m, Configure{
		DestinationCID: opened.DestinationCID,
		Options:        []Option{MTUOption(672), {Type: 0x7f, Value: []byte{1}}},
	}.Command(2)))

	got, _ := ParseConfigured(out)
	if got.Result != ConfigUnknown {
		t.Fatalf("result %#x, want the unknown options refusal", got.Result)
	}
	if len(got.Options) != 1 || got.Options[0].Type != 0x7f {
		t.Errorf("the refusal named %+v, want just the option we could not answer", got.Options)
	}
	if m.Channel(opened.DestinationCID).theirs {
		t.Error("a refused configuration was counted as done")
	}
}

// A hint may be skipped, and refusing one is how a configuration round never finishes.
func TestAHintedOptionDoesNotRefuseTheRound(t *testing.T) {
	m := NewManager(672)
	m.Serve(PSMAVDTP)

	opened, _ := ParseConnected(answer(t, m, Connect{PSM: PSMAVDTP, SourceCID: 0x0041}.Command(1))[0])

	out := only(t, answer(t, m, Configure{
		DestinationCID: opened.DestinationCID,
		Options:        []Option{MTUOption(672), {Type: 0x7e, Hint: true, Value: []byte{1}}},
	}.Command(2)))

	if got, _ := ParseConfigured(out); got.Result != ConfigSuccess {
		t.Errorf("a hinted option was refused: %#x", got.Result)
	}
}

func TestClosingAChannel(t *testing.T) {
	m := NewManager(672)
	m.Serve(PSMAVDTP)

	opened, _ := ParseConnected(answer(t, m, Connect{PSM: PSMAVDTP, SourceCID: 0x0041}.Command(1))[0])
	local := opened.DestinationCID

	out := only(t, answer(t, m, Disconnect{
		DestinationCID: local,
		SourceCID:      0x0041,
	}.Command(CodeDisconnectRequest, 5)))

	if out.Code != CodeDisconnectResponse {
		t.Errorf("answered with %#x, want a disconnect response", out.Code)
	}
	if got, _ := ParseDisconnect(out); got.DestinationCID != local || got.SourceCID != 0x0041 {
		t.Errorf("the answer carried %+v", got)
	}
	if m.Channel(local) != nil {
		t.Error("the channel is still there after being closed")
	}
}

// Everything is answered, including what makes no sense, because a far end waiting on a reply it
// will never get is worse than one told no.
func TestNonsenseIsAnsweredRatherThanIgnored(t *testing.T) {
	m := NewManager(672)

	t.Run("a channel that does not exist", func(t *testing.T) {
		out := only(t, answer(t, m, Configure{DestinationCID: 0x0099}.Command(1)))
		if out.Code != CodeReject {
			t.Errorf("answered with %#x, want a reject", out.Code)
		}
	})

	t.Run("a command nobody defines", func(t *testing.T) {
		out := only(t, answer(t, m, Command{Code: 0x7f, ID: 2}))
		if out.Code != CodeReject {
			t.Errorf("answered with %#x, want a reject", out.Code)
		}
	})

	t.Run("a truncated request", func(t *testing.T) {
		out := only(t, answer(t, m, Command{Code: CodeConnectRequest, ID: 3, Data: []byte{1}}))
		if out.Code != CodeReject {
			t.Errorf("answered with %#x, want a reject", out.Code)
		}
	})
}

func TestEchoIsAnswered(t *testing.T) {
	m := NewManager(672)

	out := only(t, answer(t, m, Command{Code: CodeEchoRequest, ID: 9}))
	if out.Code != CodeEchoResponse || out.ID != 9 {
		t.Errorf("echo answered with %#x id %d", out.Code, out.ID)
	}
}

// Nothing here has extended features worth reporting, and "not supported" is a legal answer where
// silence is not.
func TestInformationIsAnsweredWithNotSupported(t *testing.T) {
	m := NewManager(672)

	out := only(t, answer(t, m, Command{Code: CodeInfoRequest, ID: 4, Data: []byte{0x02, 0x00}}))
	if out.Code != CodeInfoResponse {
		t.Fatalf("answered with %#x", out.Code)
	}
	if len(out.Data) < 4 || out.Data[2] != 0x01 {
		t.Errorf("the result came back %x, want not supported", out.Data)
	}
}

// A link that drops takes its channels with it. Nothing is sent, because there is nothing left to
// send it over.
func TestForgettingALink(t *testing.T) {
	m := NewManager(672)
	m.Serve(PSMAVDTP)

	answer(t, m, Connect{PSM: PSMAVDTP, SourceCID: 0x0041}.Command(1))

	m.Forget()

	if len(m.Open()) != 0 {
		t.Error("channels survived the link going away")
	}
}

func TestSignallingOnTheWrongChannelIsRefused(t *testing.T) {
	m := NewManager(672)

	if _, err := m.Handle(Frame{CID: 0x0040, Payload: []byte{1, 2, 3, 4}}); err == nil {
		t.Error("signalling was accepted on a data channel")
	}
}
