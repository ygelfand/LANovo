package webview

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func frameMsg(id uint32, flags uint16, tiles ...Tile) []byte {
	b := []byte{msgFrame, version, 0, 0, 0, 0, byte(JPEG), 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(b[2:], id)
	binary.LittleEndian.PutUint16(b[7:], uint16(len(tiles)))
	binary.LittleEndian.PutUint16(b[9:], flags)
	for _, t := range tiles {
		h := make([]byte, 12)
		binary.LittleEndian.PutUint16(h[0:], uint16(t.X))
		binary.LittleEndian.PutUint16(h[2:], uint16(t.Y))
		binary.LittleEndian.PutUint16(h[4:], uint16(t.W))
		binary.LittleEndian.PutUint16(h[6:], uint16(t.H))
		binary.LittleEndian.PutUint32(h[8:], uint32(len(t.Data)))
		b = append(append(b, h...), t.Data...)
	}
	return b
}

func solid(w, h int, c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95})
	return buf.Bytes()
}

func TestParseFrameReadsTheWireLayout(t *testing.T) {
	msg := frameMsg(7, FlagLastOfFrame|FlagFullFrame, Tile{X: 3, Y: 4, W: 5, H: 6, Data: []byte{1, 2, 3}}, Tile{X: 300, Y: 400, W: 32, H: 32, Data: []byte{9}})
	f, err := ParseFrame(msg)
	if err != nil {
		t.Fatal(err)
	}
	if f.ID != 7 || f.Encoding != JPEG || f.Flags != FlagLastOfFrame|FlagFullFrame || len(f.Tiles) != 2 {
		t.Fatalf("frame %+v", f)
	}
	if a := f.Tiles[0]; a.X != 3 || a.Y != 4 || a.W != 5 || a.H != 6 || !bytes.Equal(a.Data, []byte{1, 2, 3}) {
		t.Errorf("first tile %+v", a)
	}
	if b := f.Tiles[1]; b.X != 300 || b.Y != 400 || !bytes.Equal(b.Data, []byte{9}) {
		t.Errorf("second tile %+v", b)
	}
	for n := range len(msg) {
		if _, err := ParseFrame(msg[:n]); err == nil {
			t.Fatalf("a frame cut to %d of %d bytes parsed", n, len(msg))
		}
	}
	huge := frameMsg(1, 0, Tile{Data: []byte{1}})
	binary.LittleEndian.PutUint32(huge[11+8:], 0xffffffff)
	if _, err := ParseFrame(huge); err == nil {
		t.Error("a tile claiming 4 GB parsed")
	}
}

func TestOutgoingPacketsMatchTheReference(t *testing.T) {
	if got := touch(Move, 2, 513, -5); !bytes.Equal(got, []byte{2, 1, 2, 2, 0x01, 0x02, 0, 0}) {
		t.Errorf("touch % x", got)
	}
	if got := openURL("ab"); !bytes.Equal(got, []byte{4, 1, 0, 0, 2, 0, 0, 0, 'a', 'b'}) {
		t.Errorf("open url % x", got)
	}
	if got := frameStats(0x0102, 0x03); !bytes.Equal(got, []byte{3, 1, 2, 1, 0, 0, 3, 0, 0, 0}) {
		t.Errorf("frame stats % x", got)
	}
	if got := keepalive(); !bytes.Equal(got, []byte{5, 1}) {
		t.Errorf("keepalive % x", got)
	}
	u, err := ParseCurrentURL([]byte{6, 1, 3, 0, 0, 0, 'x', 'y', 'z'})
	if err != nil || u != "xyz" {
		t.Errorf("current url %q %v", u, err)
	}
}

func TestURICarriesTheOptions(t *testing.T) {
	for _, server := range []string{"10.0.0.2:8081", "ws://10.0.0.2:8081", "ws://10.0.0.2:8081/"} {
		got, err := URI(server, Options{ID: "lanovo-1", Width: 960, Height: 600, JPEGQuality: 80, FullFrameArea: 0.5})
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(got)
		q := u.Query()
		if u.Scheme != "ws" || u.Host != "10.0.0.2:8081" || u.Path != "/" || q.Get("id") != "lanovo-1" || q.Get("w") != "960" || q.Get("h") != "600" || q.Get("q") != "80" || q.Get("ffat") != "0.5" || q.Has("ts") {
			t.Errorf("%s gave %s", server, got)
		}
	}
	if _, err := URI("", Options{}); err == nil {
		t.Error("an empty server was accepted")
	}
}

func TestYUVPasteMatchesTheStandardConversion(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for _, sz := range [][2]int{{32, 32}, {33, 17}, {1, 1}, {64, 48}} {
		src := image.NewYCbCr(image.Rect(0, 0, sz[0], sz[1]), image.YCbCrSubsampleRatio420)
		for i := range src.Y {
			src.Y[i] = uint8(r.IntN(256))
		}
		for i := range src.Cb {
			src.Cb[i], src.Cr[i] = uint8(r.IntN(256)), uint8(r.IntN(256))
		}
		want, got := image.NewRGBA(image.Rect(0, 0, 100, 80)), image.NewRGBA(image.Rect(0, 0, 100, 80))
		at := image.Rect(5, 7, 5+sz[0], 7+sz[1])
		draw.Draw(want, at, src, image.Point{}, draw.Src)
		paste(got, at, src)
		if !bytes.Equal(want.Pix, got.Pix) {
			t.Fatalf("%v: the fast conversion differs from image/draw", sz)
		}
	}
}

func TestASessionPaintsTilesAndSendsTouches(t *testing.T) {
	got := make(chan []byte, 8)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("w") != "64" {
			http.Error(w, "size", 400)
			return
		}
		c, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		c.WriteMessage(websocket.BinaryMessage, frameMsg(1, 0, Tile{X: 0, Y: 0, W: 32, H: 32, Data: solid(32, 32, color.RGBA{250, 0, 0, 255})}))
		c.WriteMessage(websocket.BinaryMessage, frameMsg(1, FlagLastOfFrame, Tile{X: 32, Y: 16, W: 32, H: 16, Data: solid(32, 16, color.RGBA{0, 0, 250, 255})}))
		c.WriteMessage(websocket.BinaryMessage, []byte{msgCurrentURL, version, 4, 0, 0, 0, 'h', 'o', 'm', 'e'})
		c.WriteMessage(websocket.BinaryMessage, []byte{msgFrameStats, version, 0, 0, 0, 0, 0, 0, 0, 0})
		for {
			_, m, err := c.ReadMessage()
			if err != nil {
				return
			}
			got <- m
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Dial(ctx, strings.TrimPrefix(srv.URL, "http://"), "http://ha/lovelace/0", Options{ID: "t", Width: 64, Height: 32})
	if err != nil {
		t.Fatal(err)
	}
	frames := make(chan image.Rectangle, 4)
	pages := make(chan string, 4)
	c.OnFrame = func(r image.Rectangle) { frames <- r }
	c.OnPage = func(u string) { pages <- u }
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	if m := <-got; m[0] != msgOpenURL || string(m[8:]) != "http://ha/lovelace/0" {
		t.Fatalf("first message % x", m)
	}
	select {
	case r := <-frames:
		if r != image.Rect(0, 0, 64, 32) {
			t.Errorf("frame dirtied %v", r)
		}
	case <-ctx.Done():
		t.Fatal("no frame")
	}
	c.View(func(img *image.RGBA) {
		if p := img.RGBAAt(10, 10); p.R < 230 || p.B > 20 {
			t.Errorf("the red tile reads %v", p)
		}
		if p := img.RGBAAt(40, 20); p.B < 230 || p.R > 20 {
			t.Errorf("the blue tile reads %v", p)
		}
		if p := img.RGBAAt(40, 5); p != (color.RGBA{}) {
			t.Errorf("an untouched pixel reads %v", p)
		}
	})
	if u := <-pages; u != "home" || c.Page() != "home" {
		t.Errorf("page %q", u)
	}
	if m := <-got; m[0] != msgFrameStats || len(m) != 10 {
		t.Errorf("stats answer % x", m)
	}
	if err := c.Touch(Down, 12, 34); err != nil {
		t.Fatal(err)
	}
	if m := <-got; !bytes.Equal(m, []byte{msgTouch, version, byte(Down), 0, 12, 0, 34, 0}) {
		t.Errorf("touch % x", m)
	}
	cancel()
	if err := <-done; err != context.Canceled {
		t.Errorf("Run ended with %v", err)
	}
}

func BenchmarkDecodeFullTile(b *testing.B) {
	for _, sz := range [][2]int{{1920, 1200}, {960, 600}} {
		img := image.NewRGBA(image.Rect(0, 0, sz[0], sz[1]))
		r := rand.New(rand.NewPCG(3, 3))
		for y := 0; y < sz[1]; y += 40 {
			draw.Draw(img, image.Rect(0, y, sz[0], y+40), &image.Uniform{color.RGBA{uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), 255}}, image.Point{}, draw.Src)
			for x := 0; x < sz[0]; x += 90 {
				draw.Draw(img, image.Rect(x+10, y+10, x+60, y+22), &image.Uniform{color.White}, image.Point{}, draw.Src)
			}
		}
		var buf bytes.Buffer
		jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85})
		data := buf.Bytes()
		canvas := image.NewRGBA(img.Bounds())
		b.Run(image.Pt(sz[0], sz[1]).String(), func(b *testing.B) {
			b.ReportMetric(float64(len(data)), "jpeg-bytes")
			for b.Loop() {
				d, err := jpeg.Decode(bytes.NewReader(data))
				if err != nil {
					b.Fatal(err)
				}
				paste(canvas, canvas.Bounds(), d)
			}
		})
	}
}
