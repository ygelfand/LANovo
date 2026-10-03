package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/lib/cast"
	"github.com/ygelfand/LANovo/internal/lib/cast/playback"
)

func TestLoungeValuesReadAsStrings(t *testing.T) {
	got := fields(json.RawMessage(`{"videoId":"Y7nDX15KJ5M","currentIndex":"13","currentTime":0,"flag":true,"none":null}`))
	want := map[string]string{"videoId": "Y7nDX15KJ5M", "currentIndex": "13", "currentTime": "0", "flag": "true"}
	if len(got) != len(want) {
		t.Errorf("read %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s reads %q, want %q", k, got[k], v)
		}
	}
	if len(fields(json.RawMessage(`nonsense`))) != 0 {
		t.Error("an unreadable payload yielded fields")
	}
}

func TestPlaylistNavigationStaysInsideTheList(t *testing.T) {
	s := &station{ids: []string{"a", "b", "c"}, index: 0}
	if prev, next := s.around(); prev || !next {
		t.Errorf("at the start: previous %v next %v", prev, next)
	}
	s.step(-1)
	if s.index != 0 {
		t.Errorf("stepping back from the start moved to %d", s.index)
	}

	s.index = 2
	if prev, next := s.around(); !prev || next {
		t.Errorf("at the end: previous %v next %v", prev, next)
	}
	s.step(1)
	if s.index != 2 {
		t.Errorf("stepping past the end moved to %d", s.index)
	}
}

func TestAPlayOrPauseWhileOpeningIsKeptForTheNextTrack(t *testing.T) {
	s := &station{opening: func() {}}
	s.pause()
	if !s.held {
		t.Error("a pause while the next track opened was lost")
	}
	s.play()
	if s.held {
		t.Error("a play while the next track opened did not undo the pause")
	}
	if !s.whileOpening(false) {
		t.Error("with a track opening and none loaded, a play was not kept for it")
	}
	s.cur = &track{}
	if s.whileOpening(true) {
		t.Error("with a track loaded, a pause was kept for later instead of applied")
	}
}

type refusals struct {
	counter
	mu     sync.Mutex
	played bool
	failed []error
}

func (r *refusals) Play(playback.Source) { r.played = true }

func (r *refusals) Failed(err error) {
	r.mu.Lock()
	r.failed = append(r.failed, err)
	r.mu.Unlock()
}

func (r *refusals) failures() []error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.failed)
}

func TestYouTubeTVWithoutACredentialTellsTheCard(t *testing.T) {
	out := &refusals{}
	s := newStation(context.Background(), ThemeTV, cast.Env{Output: out})
	s.markAsked("32EAB1DF")
	s.setPlaylist(map[string]string{"videoId": "YK51usOpBzc"})

	for i := 0; i < 100 && len(out.failures()) == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if f := out.failures(); out.played || len(f) != 1 || !strings.Contains(f[0].Error(), "no credential") {
		t.Errorf("played %v, the card was told %v", out.played, f)
	}
	if got := s.logo(); got != cast.Lookup("32EAB1DF").Icon {
		t.Errorf("the card shows %q", got)
	}
	if _, _, ok := (&screens{id: config.YouTube{Video: "v"}}).screen("32EAB1DF"); !ok {
		t.Error("YouTube TV was given no screen to connect to")
	}
}

func TestTimesAreSecondsToTheMillisecond(t *testing.T) {
	if got := secs(173114 * time.Millisecond); got != "173.114" {
		t.Errorf("read %s", got)
	}
}

// counter is an output that reads a source to its end without making a sound.
type counter struct {
	mu      sync.Mutex
	samples int
	err     error
	done    chan struct{}
}

func (c *counter) Play(src playback.Source) {
	go func() {
		defer close(c.done)
		buf := make([]int16, 1920)
		for {
			n, err := src.Read(buf)
			c.mu.Lock()
			c.samples += n
			c.err = err
			c.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
}
func (c *counter) Stop(playback.Source)                 {}
func (c *counter) Changed(playback.Source)              {}
func (c *counter) Behind(playback.Source) time.Duration { return 0 }

func (c *counter) Handoff(src playback.Source) { c.Stop(src) }
func (c *counter) Pause(playback.Source)       {}
func (c *counter) Resume(playback.Source)      {}
func (c *counter) Attend(*playback.Session)    {}

func (c *counter) Position(src playback.Source) (time.Duration, bool) { return src.Start(), false }
func (c *counter) Failed(error)                                       {}

// Against YouTube itself, so only when asked: LANOVO_LIVE=1.
func TestAVideoPlaysThroughToItsEnd(t *testing.T) {
	if os.Getenv("LANOVO_LIVE") == "" {
		t.Skip("set LANOVO_LIVE=1 to play a real video")
	}
	out := &counter{done: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	st := newStation(ctx, ThemeMusic, cast.Env{HTTP: &http.Client{Timeout: time.Minute}, Output: out})
	st.handle(Message{Name: "setPlaylist", Payload: json.RawMessage(`{"videoId":"Y7nDX15KJ5M","currentIndex":"0","currentTime":"0"}`)})

	select {
	case <-out.done:
	case <-ctx.Done():
		t.Fatal("the video did not play through")
	}
	if !errors.Is(out.err, io.EOF) {
		t.Errorf("the source ended with %v, want the end of the stream", out.err)
	}
	got := time.Duration(out.samples/channels) * time.Second / rate
	if got < 172*time.Second || got > 174*time.Second {
		t.Errorf("played %s, want about 2m53s", got)
	}
}

func TestTheStationPublishesToCast(t *testing.T) {
	var got []*cast.Published
	s := newStation(context.Background(), ThemeTV, cast.Env{Publish: func(_ cast.Playing, p *cast.Published) { got = append(got, p) }})
	s.list, s.index = "PL1", 2
	s.reportLoading("vid", 0)
	s.report(stateStopped)
	if len(got) != 2 || got[1] != nil {
		t.Fatalf("published %+v", got)
	}
	p := got[0]
	if p.Media.ContentID != "vid" || p.Media.ContentType != "x-youtube/video" || p.State != cast.StateBuffering || p.Commands != castCommands {
		t.Errorf("loading %+v", p)
	}
	if string(p.Media.CustomData) != `{"currentIndex":2,"listId":"PL1"}` || string(p.Custom) != `{"playerState":3}` {
		t.Errorf("custom %s %s", p.Media.CustomData, p.Custom)
	}
}
