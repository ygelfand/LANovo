package poster

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ygelfand/LANovo/internal/config"
	"github.com/ygelfand/LANovo/internal/lib/immich"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

func jpegOf(w, h int, c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
	}
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95})
	return buf.Bytes()
}

func TestCoverCropsToTheScreensShape(t *testing.T) {
	for _, c := range []struct {
		src  image.Rectangle
		w, h int
		want image.Rectangle
	}{
		{image.Rect(0, 0, 1440, 1080), 1200, 1920, image.Rect(382, 0, 1057, 1080)},
		{image.Rect(0, 0, 1440, 1080), 1920, 1200, image.Rect(0, 90, 1440, 990)},
		{image.Rect(0, 0, 1080, 1440), 1920, 1200, image.Rect(0, 382, 1080, 1057)},
		{image.Rect(0, 0, 800, 1280), 1200, 1920, image.Rect(0, 0, 800, 1280)},
	} {
		got := cover(c.src, c.w, c.h)
		if got != c.want {
			t.Errorf("%v into %dx%d: %v, want %v", c.src, c.w, c.h, got, c.want)
		}
		if !got.In(c.src) {
			t.Errorf("%v reaches outside %v", got, c.src)
		}
	}
}

func TestTheScrimDarkensBehindTheClockAndNowhereElse(t *testing.T) {
	src, err := decode(jpegOf(400, 300, color.RGBA{240, 240, 240, 255}))
	if err != nil {
		t.Fatal(err)
	}
	box := image.Rect(100, 200, 300, 400)
	bg := theme.Color{R: 10, G: 10, B: 10}
	img := compose(src, 400, 600, box, bg)
	if img.Bounds() != image.Rect(0, 0, 400, 600) {
		t.Fatalf("bounds %v", img.Bounds())
	}
	mid := img.RGBAAt(200, 300)
	far := img.RGBAAt(5, 5)
	if far.R < 230 {
		t.Errorf("a corner far from the clock was touched: %v", far)
	}
	k := scrimStrength
	want := int(240 - (240-10)*k)
	if d := int(mid.R) - want; d < -3 || d > 3 {
		t.Errorf("behind the clock reads %v, want about %d", mid, want)
	}
	prev := mid.R
	for x := 200; x < 400; x += 4 {
		if r := img.RGBAAt(x, 300).R; r+1 < prev {
			t.Fatalf("the scrim darkens again moving out, at x=%d: %d after %d", x, r, prev)
		} else {
			prev = r
		}
	}
}

func TestBilinearKeepsFlatColourAndBlendsAnEdge(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := range 2 {
		for x := range 4 {
			v := uint8(0)
			if x >= 2 {
				v = 200
			}
			i := src.PixOffset(x, y)
			src.Pix[i], src.Pix[i+1], src.Pix[i+2], src.Pix[i+3] = v, 50, v, 255
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, 40, 20))
	bilinear(dst, src)
	if c := dst.RGBAAt(0, 10); c.R != 0 || c.G != 50 {
		t.Errorf("left edge %v", c)
	}
	if c := dst.RGBAAt(39, 10); c.R != 200 || c.G != 50 {
		t.Errorf("right edge %v", c)
	}
	prev := uint8(0)
	for x := range 40 {
		c := dst.RGBAAt(x, 5)
		if c.R < prev || c.G != 50 || c.A != 255 {
			t.Fatalf("x=%d %v after %d", x, c, prev)
		}
		prev = c.R
	}
	if c := dst.RGBAAt(20, 10); c.R < 60 || c.R > 140 {
		t.Errorf("the edge is not blended: %v", c)
	}
}

func scrimEveryPixel(img *image.RGBA, box image.Rectangle, bg theme.Color) {
	side := float64(min(box.Dx(), box.Dy()))
	pad, soft := side*scrimPad, side*scrimSoft
	r := pad + soft*0.5
	cx, cy := float64(box.Min.X+box.Max.X)/2, float64(box.Min.Y+box.Max.Y)/2
	hw, hh := float64(box.Dx())/2+pad-r, float64(box.Dy())/2+pad-r
	strength := scrimStrength
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			qx, qy := math.Abs(float64(x)+0.5-cx)-hw, math.Abs(float64(y)+0.5-cy)-hh
			d := math.Hypot(max(qx, 0), max(qy, 0)) + min(max(qx, qy), 0) - r
			k := int32(strength*256 + 0.5)
			if d > 0 {
				t := 1 - d/soft
				if t <= 0 {
					continue
				}
				k = int32(strength*t*t*(3-2*t)*256 + 0.5)
			}
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2] = mix(img.Pix[i], bg.R, k), mix(img.Pix[i+1], bg.G, k), mix(img.Pix[i+2], bg.B, k)
		}
	}
}

func TestTheScrimMatchesAPixelAtATime(t *testing.T) {
	for _, box := range []image.Rectangle{
		image.Rect(100, 200, 300, 400), image.Rect(0, 0, 400, 300), image.Rect(-50, 450, 120, 700), image.Rect(137, 91, 158, 330),
	} {
		want, got := stripesRGBA(400, 600), stripesRGBA(400, 600)
		bg := theme.Color{R: 240, G: 20, B: 90}
		scrimEveryPixel(want, box, bg)
		scrim(got, box, bg)
		if !bytes.Equal(want.Pix, got.Pix) {
			t.Errorf("box %v: the scrim differs from a pixel at a time", box)
		}
	}
}

func stripesRGBA(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = uint8(i * 7)
	}
	return img
}

func TestAPickAvoidsTheLastPictureAndVideos(t *testing.T) {
	found := []immich.Asset{{ID: "v", Type: "VIDEO"}, {ID: "a", Type: "IMAGE"}, {ID: "b", Type: "IMAGE"}}
	if got := pick(found, "a"); got != "b" {
		t.Errorf("picked %q after a", got)
	}
	if got := pick(found[:2], "a"); got != "a" {
		t.Errorf("with only the last one left, picked %q", got)
	}
	if got := pick(found[:1], ""); got != "" {
		t.Errorf("picked the video %q", got)
	}
}

func TestDarkHasHysteresis(t *testing.T) {
	var s atomic.Bool
	for _, c := range []struct {
		lux  float64
		ok   bool
		want bool
	}{{50, true, false}, {2, true, true}, {5, true, true}, {9, true, false}, {5, true, false}, {1, true, true}, {1, false, false}} {
		if got := settle(&s, c.lux, c.ok); got != c.want {
			t.Errorf("%v lux (known %v): dark %v, want %v", c.lux, c.ok, got, c.want)
		}
	}
}

func BenchmarkCompose(b *testing.B) {
	data := jpegOf(1440, 1080, color.RGBA{120, 140, 160, 255})
	for _, sz := range [][2]int{{1920, 1200}, {1200, 1920}} {
		b.Run(image.Pt(sz[0], sz[1]).String(), func(b *testing.B) {
			box := image.Rect(sz[0]/6, sz[1]/4, sz[0]*5/6, sz[1]*3/4)
			for b.Loop() {
				src, err := decode(data)
				if err != nil {
					b.Fatal(err)
				}
				compose(src, sz[0], sz[1], box, theme.Color{R: 10, G: 12, B: 20})
			}
		})
	}
}

func testPoster(t *testing.T, fetch func(context.Context, config.Poster) (string, []byte, error)) *Poster {
	config.Use(filepath.Join(t.TempDir(), "state.json"))
	p := Get()
	p.mu.Lock()
	p.src, p.shownAt, p.failed, p.lastErr, p.made, p.madeFor = nil, time.Time{}, time.Time{}, "", nil, ""
	p.mu.Unlock()
	p.fetch = fetch
	t.Cleanup(func() { p.fetch = fetchFromImmich })
	return p
}

func TestTheScheduleFetchesWhenDueAndBacksOffAfterAFailure(t *testing.T) {
	var calls int
	var fail bool
	pic := jpegOf(64, 48, color.RGBA{200, 50, 50, 255})
	p := testPoster(t, func(context.Context, config.Poster) (string, []byte, error) {
		calls++
		if fail {
			return "", nil, errors.New("down")
		}
		return "id" + string(rune('0'+calls)), pic, nil
	})
	ctx := context.Background()
	t0 := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

	p.step(ctx, t0, false, false)
	if calls != 0 {
		t.Fatalf("fetched %d times while off", calls)
	}
	config.Set().Poster().Enabled(true)
	p.step(ctx, t0, false, false)
	if calls != 0 {
		t.Fatalf("fetched %d times with no server", calls)
	}
	config.Set().Poster().Server("http://immich")
	p.step(ctx, t0, false, false)
	if calls != 1 || config.Get().Poster.Last != "id1" {
		t.Fatalf("first step: %d calls, last %q", calls, config.Get().Poster.Last)
	}
	if img, key := p.Backdrop(40, 60, image.Rect(10, 10, 30, 30), theme.Color{}); img == nil || key == "" {
		t.Fatal("no backdrop after a fetch")
	}
	p.step(ctx, t0.Add(59*time.Minute), false, false)
	if calls != 1 {
		t.Fatalf("hourly fetched again after 59 minutes")
	}
	p.step(ctx, t0.Add(61*time.Minute), false, false)
	if calls != 2 {
		t.Fatalf("hourly did not fetch after 61 minutes")
	}

	fail = true
	t1 := t0.Add(3 * time.Hour)
	p.step(ctx, t1, false, false)
	p.step(ctx, t1.Add(time.Minute), false, false)
	if calls != 4-1 {
		t.Fatalf("retried within the backoff: %d calls", calls)
	}
	if img, _ := p.Backdrop(40, 60, image.Rectangle{}, theme.Color{}); img == nil {
		t.Error("a failed fetch dropped the picture already shown")
	}
	p.step(ctx, t1.Add(retryAfter+time.Second), false, false)
	if calls != 4 {
		t.Fatalf("did not retry after the backoff: %d calls", calls)
	}
	p.step(ctx, t1.Add(retryAfter+2*time.Second), true, false)
	if calls != 5 {
		t.Fatalf("a kick did not fetch through the backoff: %d calls", calls)
	}

	fail = false
	config.Set().Poster().Every(config.PosterWake)
	p.step(ctx, t1.Add(time.Hour), true, false)
	n := calls
	p.step(ctx, t1.Add(30*time.Hour), false, false)
	if calls != n {
		t.Fatalf("on-wake fetched without a wake")
	}
	p.step(ctx, t1.Add(30*time.Hour), false, true)
	if calls != n+1 {
		t.Fatalf("on-wake did not fetch on a wake")
	}

	config.Set().Poster().Enabled(false)
	p.step(ctx, t1.Add(31*time.Hour), false, false)
	if img, _ := p.Backdrop(40, 60, image.Rectangle{}, theme.Color{}); img != nil {
		t.Error("turning it off left the picture up")
	}
}
