package youtube

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/cast"
)

type stalling struct{ first string }

func (s stalling) RoundTrip(req *http.Request) (*http.Response, error) {
	r, w := io.Pipe()
	go func() {
		io.WriteString(w, s.first)
		<-req.Context().Done()
		w.CloseWithError(req.Context().Err())
	}()
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: r, Header: http.Header{}}, nil
}

func TestAPollThatGoesSilentEndsAsQuiet(t *testing.T) {
	was := Quiet
	Quiet = 150 * time.Millisecond
	defer func() { Quiet = was }()

	s := &Session{long: &http.Client{Transport: stalling{first: "15\n[[4,[\"noop\"]]]\n"}}, sid: "S", gid: "G"}
	var got []string
	start := time.Now()
	err := s.Poll(context.Background(), func(m Message) { got = append(got, m.Name) })
	if !errors.Is(err, ErrQuiet) {
		t.Fatalf("err %v", err)
	}
	if len(got) != 1 || got[0] != "noop" || s.aid != 4 {
		t.Errorf("got %v aid %d", got, s.aid)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
}

type recording struct{ forms []url.Values }

func (r *recording) RoundTrip(req *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(req.Body)
	v, _ := url.ParseQuery(string(b))
	r.forms = append(r.forms, v)
	return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
}

func TestTheScreenNamesItsCastDevice(t *testing.T) {
	rec := &recording{}
	dev := cast.Device{ID: "266a1baa915744d5e2531b227c645271"}
	st := &station{ctx: context.Background(), env: cast.Env{Device: dev}}
	st.bind(&Session{short: &http.Client{Transport: rec}, screen: Screen{Device: "lounge-uuid"}})

	st.discovery()
	if len(rec.forms) != 1 {
		t.Fatalf("%d posts", len(rec.forms))
	}
	f := rec.forms[0]
	if f.Get("req0__sc") != "setDiscoveryDeviceId" ||
		f.Get("req0_discoveryDeviceId") != "266A1BAA915744D5E2531B227C645271" ||
		f.Get("req0_loungeDeviceId") != "lounge-uuid" ||
		f.Get("req0_castCloudDeviceId") != dev.CloudID() {
		t.Errorf("sent %v", f)
	}
}

func TestTheServersDiscoveryEchoIsNotAnswered(t *testing.T) {
	rec := &recording{}
	st := &station{ctx: context.Background(), env: cast.Env{Device: cast.Device{ID: "266a1baa915744d5e2531b227c645271"}}}
	st.bind(&Session{short: &http.Client{Transport: rec}})

	st.handle(Message{Name: "onSetDiscoveryDeviceId"})
	if len(rec.forms) != 0 {
		t.Errorf("answered with %v", rec.forms)
	}
}

func TestNothingPlayingReportsStateZero(t *testing.T) {
	rec := &recording{}
	st := &station{ctx: context.Background()}
	st.bind(&Session{short: &http.Client{Transport: rec}})

	st.report(st.state())
	if len(rec.forms) != 1 {
		t.Fatalf("%d posts", len(rec.forms))
	}
	f := rec.forms[0]
	if f.Get("req0__sc") != "nowPlaying" || f.Get("req0_videoId") != "" ||
		f.Get("req1__sc") != "onStateChange" || f.Get("req1_state") != "0" || f.Get("req1_playabilityStatus") != "OK" {
		t.Errorf("sent %v", f)
	}
}

func TestACancelledPollIsNotQuiet(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{long: &http.Client{Transport: stalling{first: "15\n[[4,[\"noop\"]]]\n"}}, sid: "S", gid: "G"}
	err := s.Poll(ctx, func(Message) { cancel() })
	if errors.Is(err, ErrQuiet) {
		t.Fatalf("err %v", err)
	}
}
