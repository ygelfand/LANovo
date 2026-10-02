package youtube

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/cast/playback"
)

func TestSegmentsAsksByHashPrefixAndKeepsWhatWasAskedFor(t *testing.T) {
	var asked *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r
		_, _ = w.Write([]byte(`[
			{"videoID":"someoneElse","segments":[{"category":"sponsor","actionType":"skip","segment":[1,2]}]},
			{"videoID":"dQw4w9WgXcQ","segments":[
				{"category":"sponsor","actionType":"skip","segment":[30,45.5]},
				{"category":"sponsor","actionType":"skip","segment":[40,50]},
				{"category":"sponsor","actionType":"mute","segment":[60,70]},
				{"category":"filler","actionType":"skip","segment":[80,90]},
				{"category":"sponsor","actionType":"skip","segment":[5,3]},
				{"category":"sponsor","actionType":"skip","segment":[0,4]}
			]}
		]`))
	}))
	defer srv.Close()

	got, err := Segments(context.Background(), srv.Client(), srv.URL, "dQw4w9WgXcQ", []string{"sponsor"})
	if err != nil {
		t.Fatal(err)
	}

	if asked.URL.Path != "/5f6b" {
		t.Errorf("asked %s, want the hash prefix /5f6b", asked.URL.Path)
	}
	q := asked.URL.Query()
	if !slices.Equal(q["category"], []string{"sponsor"}) || q.Get("actionType") != "skip" || q.Get("service") != "YouTube" {
		t.Errorf("asked with %v", q)
	}

	want := []Segment{{0, 4 * time.Second, "sponsor"}, {30 * time.Second, 50 * time.Second, "sponsor"}}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSegmentsForAnUnknownVideoAreNone(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	got, err := Segments(context.Background(), srv.Client(), srv.URL, "x", []string{"sponsor"})
	if err != nil || got != nil {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestNoCategoriesAsksNothing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("asked with no categories")
	}))
	defer srv.Close()

	if got, err := Segments(context.Background(), srv.Client(), srv.URL, "x", nil); err != nil || got != nil {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestPlayingIntoASegmentJumpsToItsEnd(t *testing.T) {
	segs := []Segment{{From: 10 * time.Second, To: 20 * time.Second}}

	var hit []time.Duration
	for at := time.Duration(0); at < 25*time.Second; at += 5 * time.Second {
		if s, ok := inside(segs, 0, at); ok {
			hit = append(hit, at)
			if s.To != 20*time.Second {
				t.Errorf("jump to %v", s.To)
			}
		}
	}
	if want := []time.Duration{10 * time.Second, 15 * time.Second}; !slices.Equal(hit, want) {
		t.Errorf("hit at %v, want %v", hit, want)
	}
	if _, ok := inside(segs, 20*time.Second, 20*time.Second); ok {
		t.Error("the track opened at the jump's end jumped again")
	}
}

func TestStartingInsideASegmentPlaysIt(t *testing.T) {
	segs := []Segment{{From: 10 * time.Second, To: 20 * time.Second}}
	for _, at := range []time.Duration{12 * time.Second, 15 * time.Second} {
		if _, ok := inside(segs, 12*time.Second, at); ok {
			t.Errorf("a seek to 12s jumped at %v", at)
		}
	}
}

func TestASegmentAtTheStartIsSkippedFromTheBeginning(t *testing.T) {
	if s, ok := inside([]Segment{{From: 0, To: 8 * time.Second}}, 0, 0); !ok || s.To != 8*time.Second {
		t.Errorf("an intro at 0: %v %v", s, ok)
	}
}

func TestATrackWaitingOnAJumpHandsOverNothing(t *testing.T) {
	tr := &track{jumping: true}
	if n, err := tr.Read(make([]int16, 64)); n != 0 || err != nil {
		t.Errorf("read %d, %v", n, err)
	}
}

func TestSegmentsBecomeMarksInSponsorBlockColours(t *testing.T) {
	got := marks([]Segment{{From: time.Second, To: 2 * time.Second, Category: "sponsor"}, {From: 5 * time.Second, To: 6 * time.Second, Category: "intro"}})
	want := []playback.Mark{{From: time.Second, To: 2 * time.Second, RGB: 0x00d400}, {From: 5 * time.Second, To: 6 * time.Second, RGB: 0x00ffff}}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCategoriesAreCleanedUp(t *testing.T) {
	got := ParseCategories(" Sponsor, selfpromo,sponsr  intro,sponsor\tmusic_offtopic ")
	if want := []string{"sponsor", "selfpromo", "intro", "music_offtopic"}; !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := ParseCategories(""); got != nil {
		t.Errorf("empty gave %v", got)
	}
}
