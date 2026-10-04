package youtube

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/cast/playback"

	yt "github.com/kkdai/youtube/v2"

	"github.com/ygelfand/LANovo/internal/lib/webm"
)

func TestAVideoStreamIsVP9(t *testing.T) {
	if os.Getenv("LANOVO_LIVE") == "" {
		t.Skip("set LANOVO_LIVE=1 to ask YouTube")
	}
	r := NewResolver(&http.Client{Timeout: time.Minute}, nil)
	_, stream, err := r.Video(t.Context(), "Y7nDX15KJ5M")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	head := make([]byte, 4<<20)
	n, err := io.ReadFull(stream, head)
	if err != nil && n == 0 {
		t.Fatal(err)
	}
	head = head[:n]
	if cut := bytes.LastIndex(head, []byte{0x1F, 0x43, 0xB6, 0x75}); cut > 0 {
		head = head[:cut]
	}
	tr, err := webm.NewVideoReader(bytes.NewReader(head)).Track()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s %dx%d, %d bytes kept", tr.Codec, tr.Width, tr.Height, len(head))
	if tr.Codec != "V_VP9" || tr.Height == 0 {
		t.Errorf("track %+v", tr)
	}
	if dump := os.Getenv("LANOVO_DUMP"); dump != "" {
		if err := os.WriteFile(dump, head, 0o644); err != nil {
			t.Error(err)
		}
	}
}

func TestTheBestOpusFormatIsChosen(t *testing.T) {
	formats := yt.FormatList{
		{ItagNo: 140, MimeType: `audio/mp4; codecs="mp4a.40.2"`, Bitrate: 131104},
		{ItagNo: 249, MimeType: `audio/webm; codecs="opus"`, Bitrate: 55420},
		{ItagNo: 251, MimeType: `audio/webm; codecs="opus"`, Bitrate: 147426},
		{ItagNo: 248, MimeType: `video/webm; codecs="vp9"`, Bitrate: 2000000},
	}
	if f := bestOpus(formats); f == nil || f.ItagNo != 251 {
		t.Errorf("chose %+v", f)
	}
	if f := bestOpus(formats[:1]); f != nil {
		t.Errorf("chose %d from a list with no opus", f.ItagNo)
	}
}

func TestTheOriginalLanguageIsChosenOverADubOrAProcessedCopy(t *testing.T) {
	const opus = `audio/webm; codecs="opus"`
	at := func(xtags string) string {
		return "https://gv.example/videoplayback?itag=251&xtags=" + url.QueryEscape(xtags)
	}
	var def, dub yt.Format
	def.AudioTrack = &struct {
		DisplayName    string `json:"displayName"`
		ID             string `json:"id"`
		AudioIsDefault bool   `json:"audioIsDefault"`
	}{AudioIsDefault: true}
	dub.AudioTrack = &struct {
		DisplayName    string `json:"displayName"`
		ID             string `json:"id"`
		AudioIsDefault bool   `json:"audioIsDefault"`
	}{}
	format := func(track yt.Format, bitrate int, xtags string) yt.Format {
		track.ItagNo, track.MimeType, track.Bitrate, track.URL = 251, opus, bitrate, at(xtags)
		return track
	}

	for _, c := range []struct {
		name    string
		formats yt.FormatList
		want    int
	}{
		{"a louder dub, and processed copies of the original", yt.FormatList{
			format(dub, 158163, "acont=dubbed:lang=es-US"),
			format(def, 150000, "acont=original:lang=en-US:drc=1"),
			format(def, 140000, "acont=original:lang=en-US:vb=1"),
			format(def, 128000, "acont=original:lang=en-US"),
		}, 128000},
		{"only the default flag to go on", yt.FormatList{
			format(dub, 158163, ""),
			format(def, 120000, ""),
		}, 120000},
		{"one language, one of them compressed", yt.FormatList{
			format(yt.Format{}, 124446, "drc=1"),
			format(yt.Format{}, 112788, ""),
		}, 112788},
	} {
		if f := bestOpus(c.formats); f == nil || f.Bitrate != c.want {
			t.Errorf("%s: chose %+v, want the one at %d", c.name, f, c.want)
		}
	}
}

func TestTheTallestVP9ThatTheDecoderKeepsUpWithIsChosen(t *testing.T) {
	formats := yt.FormatList{
		{ItagNo: 251, MimeType: `audio/webm; codecs="opus"`, Bitrate: 147426},
		{ItagNo: 137, MimeType: `video/mp4; codecs="avc1.640028"`, Height: 1080, FPS: 30, Bitrate: 4000000},
		{ItagNo: 247, MimeType: `video/webm; codecs="vp9"`, Height: 720, FPS: 30, Bitrate: 1500000},
		{ItagNo: 248, MimeType: `video/webm; codecs="vp9"`, Height: 1080, FPS: 30, Bitrate: 2500000},
		{ItagNo: 303, MimeType: `video/webm; codecs="vp9"`, Height: 1080, FPS: 60, Bitrate: 4000000},
		{ItagNo: 271, MimeType: `video/webm; codecs="vp9"`, Height: 1440, FPS: 30, Bitrate: 9000000},
	}
	if f := bestVP9(formats, playback.Target{Tallest: 1080, Fastest: 60}); f == nil || f.ItagNo != 303 {
		t.Errorf("chose %+v", f)
	}
	if f := bestVP9(yt.FormatList{formats[4]}, playback.Target{Tallest: 1080, Fastest: 60}); f == nil || f.ItagNo != 303 {
		t.Errorf("with only 60 fps on offer, chose %+v", f)
	}
	if f := bestVP9(formats[:2], playback.Target{Tallest: 1080, Fastest: 60}); f != nil {
		t.Errorf("chose %d from a list with no vp9", f.ItagNo)
	}
}

func TestAChunkIsFifteenSecondsOfItsStream(t *testing.T) {
	for _, at := range []struct {
		f    yt.Format
		want int64
	}{
		{yt.Format{ItagNo: 251, ContentLength: 3012048, ApproxDurationMs: "173121"}, 260970},
		{yt.Format{ItagNo: 248, ContentLength: 6934788, ApproxDurationMs: "173080"}, 600990},
		{yt.Format{ItagNo: 1, ContentLength: 3012048}, chunkFloor},
		{yt.Format{ItagNo: 2, ContentLength: 1000, ApproxDurationMs: "173121"}, chunkFloor},
	} {
		if got := chunkFor(&at.f); got != at.want {
			t.Errorf("itag %d: chunk %d, want %d", at.f.ItagNo, got, at.want)
		}
	}
}

func TestTheTallestPictureFollowsThePanel(t *testing.T) {
	formats := yt.FormatList{
		{ItagNo: 248, MimeType: `video/webm; codecs="vp9"`, Height: 1080, FPS: 30, Bitrate: 2500000},
		{ItagNo: 247, MimeType: `video/webm; codecs="vp9"`, Height: 720, FPS: 30, Bitrate: 1200000},
		{ItagNo: 244, MimeType: `video/webm; codecs="vp9"`, Height: 480, FPS: 30, Bitrate: 600000},
	}
	if f := bestVP9(formats, playback.Target{Tallest: 720}); f == nil || f.ItagNo != 247 {
		t.Fatalf("a 720 line cap chose %v", f)
	}
	r := NewResolver(nil, func() playback.Target { return playback.Target{Tallest: 480} })
	if f := bestVP9(formats, r.limits()); f == nil || f.ItagNo != 244 {
		t.Fatalf("the resolver's target chose %v", f)
	}
}
