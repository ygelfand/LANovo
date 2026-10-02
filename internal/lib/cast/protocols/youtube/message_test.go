package youtube

import (
	"net/url"
	"strings"
	"testing"
	"testing/iotest"
)

func parse(t *testing.T, body string) []Message {
	t.Helper()
	msgs, err := Parse(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return msgs
}

func TestABindAnswerYieldsItsSession(t *testing.T) {
	body := "87\n[[0,[\"c\",\"8A1F2E3D\",\"\",8]],[1,[\"S\",\"gsid-123\"]],[2,[\"loungeStatus\",{\"devices\":\"[]\"}]]]\n"
	msgs := parse(t, body)
	if len(msgs) != 3 {
		t.Fatalf("%d messages: %+v", len(msgs), msgs)
	}
	if msgs[0].Name != "c" || msgs[0].First() != "8A1F2E3D" {
		t.Errorf("the session id reads %q", msgs[0].First())
	}
	if msgs[1].Name != "S" || msgs[1].String() != "gsid-123" {
		t.Errorf("the gsessionid reads %q", msgs[1].String())
	}
	if msgs[2].AID != 2 || msgs[2].Name != "loungeStatus" || string(msgs[2].Payload) != `{"devices":"[]"}` {
		t.Errorf("the third message is %+v", msgs[2])
	}
}

func TestAMessageWithNoPayloadParses(t *testing.T) {
	msgs := parse(t, `[[5,["noop"]]]`)
	if len(msgs) != 1 || msgs[0].Name != "noop" || msgs[0].AID != 5 || msgs[0].Payload != nil {
		t.Errorf("parsed %+v", msgs)
	}
}

func TestAPayloadHoldingBracketsSurvives(t *testing.T) {
	msgs := parse(t, "60\n[[7,[\"setPlaylist\",{\"videoId\":\"abc\",\"title\":\"a ]] b\"}]]]\n")
	if len(msgs) != 1 || msgs[0].Name != "setPlaylist" || string(msgs[0].Payload) != `{"videoId":"abc","title":"a ]] b"}` {
		t.Errorf("parsed %+v", msgs)
	}
}

func TestNonASCIIDoesNotDependOnTheLengthUnit(t *testing.T) {
	msgs := parse(t, "40\n[[8,[\"nowPlaying\",{\"title\":\"café 🎵\"}]]]\n")
	if len(msgs) != 1 || !strings.Contains(string(msgs[0].Payload), "café 🎵") {
		t.Errorf("parsed %+v", msgs)
	}
}

func TestAChunkArrivingInPiecesIsOneChunk(t *testing.T) {
	body := "30\n[[1,[\"noop\"]],[2,[\"play\"]]]\n25\n[[3,[\"pause\",{\"a\":1}]]]\n"
	var chunks [][]Message
	err := Frames(iotest.OneByteReader(strings.NewReader(body)), func(m []Message) { chunks = append(chunks, m) })
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 || len(chunks[0]) != 2 || chunks[0][1].Name != "play" || chunks[1][0].Name != "pause" {
		t.Errorf("chunks %+v", chunks)
	}
}

func TestSeveralArgumentsBecomeAnArray(t *testing.T) {
	msgs := parse(t, `[[4,["c","SID1","",8]]]`)
	if len(msgs) != 1 || string(msgs[0].Payload) != `["SID1","",8]` {
		t.Errorf("parsed %+v", msgs)
	}
}

func TestAMalformedChunkIsAnError(t *testing.T) {
	if _, err := Parse(`[[1,"noop"]]`); err == nil {
		t.Error("a message without a body parsed")
	}
}

func TestSessionLearnsItsIDsAndTheHighestAID(t *testing.T) {
	s := &Session{aid: 3}
	s.take(parse(t, `[[0,["c","SID1","",8]],[1,["S","GID1"]],[9,["noop"]]]`))
	if s.sid != "SID1" || s.gid != "GID1" || s.aid != 9 {
		t.Errorf("sid %q gid %q aid %d", s.sid, s.gid, s.aid)
	}
}

func TestOutgoingMessagesAreNumberedFromTheOffset(t *testing.T) {
	v := form(4, []Out{
		{Name: "nowPlaying"},
		{Name: "onVolumeChanged", Fields: map[string]string{"volume": "45", "muted": "false"}},
	})
	want := url.Values{
		"count": {"2"}, "ofs": {"4"},
		"req0__sc": {"nowPlaying"},
		"req1__sc": {"onVolumeChanged"}, "req1_volume": {"45"}, "req1_muted": {"false"},
	}
	if v.Encode() != want.Encode() {
		t.Errorf("got %s, want %s", v.Encode(), want.Encode())
	}
}
