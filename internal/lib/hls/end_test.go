package hls

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
)

func packedSegment(t *testing.T, pts uint64) []byte {
	t.Helper()
	adts, err := mpeg4audio.ADTSPackets{
		{Type: mpeg4audio.ObjectTypeAACLC, SampleRate: 48000, ChannelConfig: 2, AU: []byte{0x21, 0x10, 0x04, 0x60, 0x8c, 0x1c}},
	}.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	priv := []byte(tsTimestampOwner + "\x00")
	for i := 7; i >= 0; i-- {
		priv = append(priv, byte(pts>>(8*i)))
	}
	frame := append([]byte("PRIV"), 0, 0, 0, byte(len(priv)), 0, 0)
	frame = append(frame, priv...)
	tag := append([]byte("ID3\x04\x00\x00"), 0, 0, 0, byte(len(frame)))
	return append(append(tag, frame...), adts...)
}

type fakeLive struct {
	t        *testing.T
	segments int
	ended    bool

	mu      sync.Mutex
	fetched []int
}

func (f *fakeLive) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/audio.m3u8" {
		var b strings.Builder
		b.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:1\n#EXT-X-MEDIA-SEQUENCE:0\n")
		for i := range f.segments {
			fmt.Fprintf(&b, "#EXTINF:1.0,\n/sq/%d/seg.ts\n", i)
		}
		if f.ended {
			b.WriteString("#EXT-X-ENDLIST\n")
		}
		w.Write([]byte(b.String()))
		return
	}
	var n int
	if _, err := fmt.Sscanf(r.URL.Path, "/sq/%d/seg.ts", &n); err != nil || n >= f.segments {
		http.NotFound(w, r)
		return
	}
	f.mu.Lock()
	f.fetched = append(f.fetched, n)
	f.mu.Unlock()
	w.Write(packedSegment(f.t, uint64(n)*tsClock))
}

func followed(t *testing.T, f *fakeLive, within time.Duration) error {
	t.Helper()
	least, targets := stallLeast, stallTargets
	stallLeast, stallTargets = 500*time.Millisecond, 0
	t.Cleanup(func() { stallLeast, stallTargets = least, targets })

	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	s := &Stream{cancel: cancel, done: make(chan struct{}), sized: make(chan struct{}), anchored: make(chan struct{}), frames: make(chan Frame, 1)}
	s.room = sync.NewCond(&s.mu)

	got := make(chan error, 1)
	go func() { got <- s.follow(ctx, srv.URL+"/audio.m3u8", false, true) }()
	select {
	case err := <-got:
		s.fail(err)
		buf := make([]int16, 4096)
		for range 1000 {
			n, rerr := s.Read(buf)
			if rerr != nil {
				if !errors.Is(rerr, io.EOF) {
					t.Errorf("read after the end gave %v", rerr)
				}
				return err
			}
			if n == 0 {
				t.Fatal("read gave nothing and no end")
			}
		}
		t.Fatal("read never ended")
		return err
	case <-time.After(within):
		cancel()
		t.Fatalf("still following after %v", within)
		return nil
	}
}

func TestAnEndedPlaylistEndsTheStream(t *testing.T) {
	f := &fakeLive{t: t, segments: 3, ended: true}
	if err := followed(t, f, 3*time.Second); !errors.Is(err, io.EOF) {
		t.Fatalf("ended with %v", err)
	}
	if len(f.fetched) != 3 {
		t.Errorf("fetched %v", f.fetched)
	}
}

func TestAStreamThatGoesQuietEnds(t *testing.T) {
	f := &fakeLive{t: t, segments: 3}
	if err := followed(t, f, 5*time.Second); !errors.Is(err, io.EOF) {
		t.Fatalf("ended with %v", err)
	}
	if len(f.fetched) != 3 {
		t.Errorf("fetched %v", f.fetched)
	}
}
