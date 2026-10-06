package rtc

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"
)

type tone struct {
	freq float64

	mu    sync.Mutex
	heard []int16
}

func (t *tone) Frames() (<-chan []int16, func()) {
	ch := make(chan []int16, 8)
	ctx, stop := context.WithCancel(context.Background())
	go func() {
		defer close(ch)
		tick := time.NewTicker(frame)
		defer tick.Stop()
		n := 0
		for {
			f := make([]int16, FrameSamples)
			for i := range f {
				f[i] = int16(8000 * math.Sin(2*math.Pi*t.freq*float64(n)/16000))
				n++
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				ch <- f
			}
		}
	}()
	return ch, stop
}

func (t *tone) Play(pcm []int16) {
	t.mu.Lock()
	t.heard = append(t.heard, pcm...)
	t.mu.Unlock()
}

func (t *tone) loudest() (int, int16) {
	t.mu.Lock()
	defer t.mu.Unlock()
	var peak int16
	for _, s := range t.heard {
		peak = max(peak, s, -s)
	}
	return len(t.heard), peak
}

func TestLinkCarriesSpeechBothWays(t *testing.T) {
	caller, callee := &tone{freq: 440}, &tone{freq: 660}
	ended := func(webrtc.PeerConnectionState) {}

	a, err := New(Media{Audio: caller}, ended)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := New(Media{Audio: callee}, ended)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	offer, err := a.Offer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	answer, err := b.Accept(ctx, offer)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Answered(answer); err != nil {
		t.Fatal(err)
	}

	want := PlayRate * PlayChannels / 2
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		na, pa := caller.loudest()
		nb, pb := callee.loudest()
		if na >= want && nb >= want && pa > 1000 && pb > 1000 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	na, pa := caller.loudest()
	nb, pb := callee.loudest()
	t.Fatalf("caller heard %d samples peak %d, callee heard %d samples peak %d", na, pa, nb, pb)
}

func TestMutedSendsSilence(t *testing.T) {
	caller, callee := &tone{freq: 440}, &tone{freq: 660}
	a, err := New(Media{Audio: caller}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := New(Media{Audio: callee}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	a.Mute(true)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	offer, err := a.Offer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	answer, err := b.Accept(ctx, offer)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Answered(answer); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if n, _ := callee.loudest(); n >= PlayRate*PlayChannels/2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	n, peak := callee.loudest()
	if n == 0 {
		t.Fatal("callee heard nothing at all")
	}
	if peak > 200 {
		t.Fatalf("muted caller reached the callee at peak %d", peak)
	}
}

type film struct {
	asked atomic.Int32
}

func (f *film) Key() { f.asked.Add(1) }

func au(types ...byte) []byte {
	var out []byte
	for n, t := range types {
		size := 40
		if t == 5 {
			size = 5000
		}
		body := make([]byte, size)
		for i := range body {
			body[i] = byte(i*7+n) | 0x01
		}
		body[0] = 0x60 | t
		out = append(out, 0, 0, 0, 1)
		out = append(out, body...)
	}
	return out
}

func (f *film) Frames() (<-chan Frame, func()) {
	ch := make(chan Frame, 8)
	ctx, stop := context.WithCancel(context.Background())
	go func() {
		defer close(ch)
		tick := time.NewTicker(33 * time.Millisecond)
		defer tick.Stop()
		for n := 0; ; n++ {
			key := n%10 == 0
			data := au(1)
			if key {
				data = au(7, 8, 5)
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				ch <- Frame{Data: data, PTS: time.Duration(n) * 33 * time.Millisecond, Key: key}
			}
		}
	}()
	return ch, stop
}

type wall struct {
	mu    sync.Mutex
	shown []Frame
}

func (w *wall) Show(f Frame) {
	w.mu.Lock()
	w.shown = append(w.shown, f)
	w.mu.Unlock()
}

func (w *wall) frames() []Frame {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]Frame(nil), w.shown...)
}

func TestLinkCarriesVideo(t *testing.T) {
	a, err := New(Media{Audio: &tone{freq: 440}, Camera: &film{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	screen := &wall{}
	b, err := New(Media{Audio: &tone{freq: 660}, Screen: screen}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	offer, err := a.Offer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	answer, err := b.Accept(ctx, offer)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Answered(answer); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && len(screen.frames()) < 15 {
		time.Sleep(100 * time.Millisecond)
	}
	got := screen.frames()
	if len(got) < 15 {
		t.Fatalf("screen showed %d frames", len(got))
	}
	if !got[0].Key {
		t.Fatal("first frame shown was not a keyframe")
	}
	want := au(7, 8, 5)
	if string(got[0].Data) != string(want) {
		t.Fatalf("keyframe arrived as %d bytes, sent %d", len(got[0].Data), len(want))
	}
	for i := 1; i < len(got); i++ {
		if got[i].PTS <= got[i-1].PTS {
			t.Fatalf("frame %d at %s after %s", i, got[i].PTS, got[i-1].PTS)
		}
	}
}

func TestCameraOffStopsVideoUntilAKeyframe(t *testing.T) {
	a, err := New(Media{Audio: &tone{freq: 440}, Camera: &film{}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.CameraOff(true)
	screen := &wall{}
	b, err := New(Media{Audio: &tone{freq: 660}, Screen: screen}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	offer, err := a.Offer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	answer, err := b.Accept(ctx, offer)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Answered(answer); err != nil {
		t.Fatal(err)
	}

	time.Sleep(1500 * time.Millisecond)
	if n := len(screen.frames()); n != 0 {
		t.Fatalf("camera off, yet %d frames were shown", n)
	}
	a.CameraOff(false)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && len(screen.frames()) < 3 {
		time.Sleep(50 * time.Millisecond)
	}
	got := screen.frames()
	if len(got) < 3 {
		t.Fatalf("camera back on, but only %d frames shown", len(got))
	}
	if !got[0].Key {
		t.Fatal("video resumed on a frame that was not a keyframe")
	}
}

type note struct {
	Text string `json:"text"`
	N    int    `json:"n"`
}

func TestMessagesGoBothWays(t *testing.T) {
	var mu sync.Mutex
	got := map[string][]Message{}
	keep := func(side string) func(Message) {
		return func(m Message) {
			mu.Lock()
			got[side] = append(got[side], m)
			mu.Unlock()
		}
	}
	opened := make(chan struct{}, 2)
	open := func() { opened <- struct{}{} }
	a, err := New(Media{Audio: &tone{freq: 440}, Opened: open, Received: keep("a")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := New(Media{Audio: &tone{freq: 660}, Opened: open, Received: keep("b")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if err := a.Send("early", note{}); !errors.Is(err, ErrNotOpen) {
		t.Fatalf("send before connecting: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	offer, err := a.Offer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	answer, err := b.Accept(ctx, offer)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Answered(answer); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case <-opened:
		case <-ctx.Done():
			t.Fatal("data channel never opened")
		}
	}

	if err := a.Send("status", note{Text: "muted", N: 1}); err != nil {
		t.Fatal(err)
	}
	if err := b.Send("hello", note{Text: "hi", N: 2}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := len(got["a"]) > 0 && len(got["b"]) > 0
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	check := func(side, kind string, want note) {
		if len(got[side]) != 1 || got[side][0].Kind != kind {
			t.Fatalf("%s got %+v, want one %q", side, got[side], kind)
		}
		var n note
		if err := json.Unmarshal(got[side][0].Data, &n); err != nil || n != want {
			t.Fatalf("%s decoded %+v (%v), want %+v", side, n, err, want)
		}
	}
	check("b", "status", note{Text: "muted", N: 1})
	check("a", "hello", note{Text: "hi", N: 2})
}

func TestPictureLossAsksTheCameraForAKeyframe(t *testing.T) {
	cam := &film{}
	a, err := New(Media{Audio: &tone{freq: 440}, Camera: cam}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	screen := &wall{}
	b, err := New(Media{Audio: &tone{freq: 660}, Screen: screen}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	offer, err := a.Offer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	answer, err := b.Accept(ctx, offer)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Answered(answer); err != nil {
		t.Fatal(err)
	}

	var ssrc uint32
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && ssrc == 0 {
		for _, r := range b.pc.GetReceivers() {
			if tr := r.Track(); tr != nil && tr.Kind() == webrtc.RTPCodecTypeVideo && len(screen.frames()) > 0 {
				ssrc = uint32(tr.SSRC())
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if ssrc == 0 {
		t.Fatal("no video arrived")
	}
	for range 20 {
		if err := b.pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: ssrc}}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	asked := cam.asked.Load()
	if asked == 0 {
		t.Fatalf("20 picture loss reports never reached the camera: %+v", a.Stats())
	}
	if asked > 3 {
		t.Fatalf("camera asked %d times for 20 reports in a second: not limited", asked)
	}
}
