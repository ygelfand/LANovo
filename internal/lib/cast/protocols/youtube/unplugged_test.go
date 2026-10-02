package youtube

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/lib/surface"
)

func TestTheTallestH264UnderTheCapAndTheDefaultAACArePicked(t *testing.T) {
	def := func(d bool) *struct {
		AudioIsDefault bool `json:"audioIsDefault"`
	} {
		return &struct {
			AudioIsDefault bool `json:"audioIsDefault"`
		}{d}
	}
	fs := []unpluggedFormat{
		{Itag: 136, MimeType: `video/mp4; codecs="avc1.4d401f"`, Height: 720, FPS: 30, Bitrate: 2_000_000},
		{Itag: 145, MimeType: `video/mp4; codecs="avc1.4d401f"`, Height: 720, FPS: 30, Bitrate: 2_600_000},
		{Itag: 137, MimeType: `video/mp4; codecs="avc1.640028"`, Height: 1080, FPS: 30, Bitrate: 5_000_000},
		{Itag: 248, MimeType: `video/webm; codecs="vp9"`, Height: 720, FPS: 30, Bitrate: 9_000_000},
		{Itag: 149, MimeType: `audio/mp4; codecs="mp4a.40.2"`, Bitrate: 144_000, AudioTrack: def(false)},
		{Itag: 148, MimeType: `audio/mp4; codecs="mp4a.40.5"`, Bitrate: 64_000, AudioTrack: def(true)},
		{Itag: 150, MimeType: `audio/mp4; codecs="mp4a.40.2"`, Bitrate: 128_000, AudioTrack: def(true)},
	}
	v, a := pickUnplugged(fs)
	if v == nil || v.Itag != 145 {
		t.Errorf("video %+v", v)
	}
	if a == nil || a.Itag != 150 {
		t.Errorf("audio %+v", a)
	}
}

func TestTheWidevinePSSHIsChosenOverOthers(t *testing.T) {
	box := func(system [16]byte) []byte {
		b := make([]byte, 32)
		copy(b[4:], "pssh")
		copy(b[12:], system[:])
		return b
	}
	playready := [16]byte{0x9a, 0x04, 0xf0, 0x79}
	wv := box(surface.Widevine)
	if got := widevinePSSH([][]byte{box(playready), wv}); !bytes.Equal(got, wv) {
		t.Errorf("chose %x", got)
	}
	if got := widevinePSSH([][]byte{box(playready)}); got != nil {
		t.Errorf("chose %x with no Widevine box", got)
	}
}

func fakeServer(t *testing.T, target *string, answer func(body map[string]any) (int, any)) *[]map[string]any {
	t.Helper()
	var mu sync.Mutex
	var seen []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(raw, &body)
		mu.Lock()
		seen = append(seen, body)
		mu.Unlock()
		code, out := answer(body)
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)
	old := *target
	*target = srv.URL
	t.Cleanup(func() { *target = old })
	return &seen
}

func tokenOf(body map[string]any) string {
	ctx, _ := body["context"].(map[string]any)
	user, _ := ctx["user"].(map[string]any)
	tokens, _ := user["credentialTransferTokens"].([]any)
	if len(tokens) == 0 {
		return ""
	}
	first, _ := tokens[0].(map[string]any)
	return first["token"].(string)
}

func TestTheLicenseRequestCarriesTheNonceAndTheCredential(t *testing.T) {
	license := []byte{0xde, 0xad, 0xbe, 0xef, 0x01}
	seen := fakeServer(t, &licenseURL, func(map[string]any) (int, any) {
		return 200, map[string]string{"status": "LICENSE_STATUS_OK", "license": base64.RawURLEncoding.EncodeToString(license)}
	})
	got, err := unpluggedLicense(context.Background(), http.DefaultClient, "ctt-1", "chan", "nonce-16-chars-x", "sess", "Cg0%3D", []byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, license) {
		t.Errorf("license %x", got)
	}
	b := (*seen)[0]
	if b["cpn"] != "nonce-16-chars-x" || b["sessionId"] != "sess" || b["drmParams"] != "Cg0=" || b["videoId"] != "chan" || tokenOf(b) != "ctt-1" {
		t.Errorf("request %v", b)
	}
	if b["licenseRequest"] != base64.StdEncoding.EncodeToString([]byte{1, 2, 3}) {
		t.Errorf("challenge %v", b["licenseRequest"])
	}
}

func TestANonceIsSixteenURLSafeCharacters(t *testing.T) {
	n := nonce()
	if len(n) != 16 || strings.Trim(n, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_") != "" {
		t.Errorf("nonce %q", n)
	}
	if nonce() == n {
		t.Error("two nonces are the same")
	}
}

func TestTheGuideReadsTheAiringAndAFreshCredential(t *testing.T) {
	fakeServer(t, &guideURL, func(map[string]any) (int, any) {
		return 200, json.RawMessage(`{
			"contents": {"singleColumnWatchNextResults": {"results": {"watchNextTabbedResultsRenderer": {"videoMetadata": {"unpluggedVideoMetadataRenderer": {
				"primaryText": {"runs": [{"text": "Donnie "}, {"text": "Brasco"}]},
				"secondaryText": {"simpleText": "R"},
				"tertiaryText": {"runs": [{"text": "Live • 11:00 PM – 2:00 AM"}]},
				"networkName": {"runs": [{"text": "SundanceTV"}]},
				"endTimeSeconds": "1790748000",
				"thumbnail": {"thumbnails": [{"url": "//small"}, {"url": "//big"}]},
				"networkIcon": {"thumbnails": [{"url": "//logo"}]}
			}}}}}},
			"currentVideoEndpoint": {"watchEndpoint": {"watchEndpointSupportedAuthorizationTokenConfig": {"videoAuthorizationToken": {"credentialTransferTokens": [{"scope": "VIDEO", "token": "ctt-2"}]}}}}
		}`)
	})
	a, err := unpluggedNext(context.Background(), http.DefaultClient, "ctt-1", "chan")
	if err != nil {
		t.Fatal(err)
	}
	want := airing{title: "Donnie Brasco", network: "SundanceTV", rating: "R", slot: "11:00 PM – 2:00 AM", art: "https://big", mark: "https://logo", ends: time.Unix(1790748000, 0), ctt: "ctt-2"}
	if a != want {
		t.Errorf("airing\n got %+v\nwant %+v", a, want)
	}
}

func bareUnplugged() *unplugged {
	_, cancel := context.WithCancel(context.Background())
	u := &unplugged{hc: http.DefaultClient, id: "chan", cpn: "nonce", ctt: "ctt-1", cancel: cancel, done: make(chan struct{})}
	u.room = sync.NewCond(&u.mu)
	return u
}

func TestARefusedHeartbeatEndsTheStreamWithTheReason(t *testing.T) {
	seen := fakeServer(t, &heartbeatURL, func(map[string]any) (int, any) {
		return 200, map[string]any{"playabilityStatus": map[string]string{"status": "UNPLAYABLE", "reason": "Too many streams"}}
	})
	u := bareUnplugged()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	u.heartbeat(ctx, "token", "data", "1")
	select {
	case <-u.done:
	default:
		t.Fatal("the stream kept going")
	}
	if err := u.failure(); err == nil || !strings.Contains(err.Error(), "too many streams") {
		t.Errorf("failure %v", err)
	}
	b := (*seen)[0]
	if b["heartbeatToken"] != "token" || b["heartbeatServerData"] != "data" || b["cpn"] != "nonce" || tokenOf(b) != "ctt-1" {
		t.Errorf("beat %v", b)
	}
}

func TestHeartbeatsCarryTheServersDataForwardAndSurviveErrors(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	seen := fakeServer(t, &heartbeatURL, func(map[string]any) (int, any) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 2 {
			return 500, map[string]string{}
		}
		return 200, map[string]any{"playabilityStatus": map[string]string{"status": "OK"}, "pollDelayMs": "1", "heartbeatServerData": "next-data"}
	})
	u := bareUnplugged()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		u.heartbeat(ctx, "token", "first-data", "1")
		close(done)
	}()
	for i := 0; i < 200; i++ {
		mu.Lock()
		n := calls
		mu.Unlock()
		if n >= 3 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	select {
	case <-u.done:
		t.Fatal("a failed beat ended the stream")
	default:
	}
	beats := *seen
	if len(beats) < 3 {
		t.Fatalf("%d beats", len(beats))
	}
	if beats[0]["heartbeatServerData"] != "first-data" || beats[2]["heartbeatServerData"] != "next-data" {
		t.Errorf("server data %v then %v", beats[0]["heartbeatServerData"], beats[2]["heartbeatServerData"])
	}
	if beats[0]["sequenceNumber"] != float64(0) || beats[2]["sequenceNumber"] != float64(2) {
		t.Errorf("sequence %v then %v", beats[0]["sequenceNumber"], beats[2]["sequenceNumber"])
	}
}
