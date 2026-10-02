package glow

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"testing"
)

func (bl *Bloom) addUpEverywhere(img *image.RGBA, o *octave, k int32) {
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()
	bl.columns(W, o)
	sy := float32(o.h) / float32(H)
	for y := range H {
		fy := (float32(y)+0.5)*sy - 0.5
		y0 := clampi(int(fy), 0, o.h-1)
		y1 := min(y0+1, o.h-1)
		wy := int32(min(max(fy-float32(y0), 0), 1) * 256)
		bl.spread(bl.rowA, o, y0, k)
		bl.spread(bl.rowB, o, y1, k)
		row := img.Pix[y*img.Stride : y*img.Stride+W*4]
		for x := range W {
			for ch := range 3 {
				v := int32(row[x*4+ch]) + ((bl.rowA[x*3+ch]*(256-wy)+bl.rowB[x*3+ch]*wy)>>8)>>4
				row[x*4+ch] = uint8(min(v, 255))
			}
		}
	}
}

func scene(w, h int, dense bool, seed uint64) *image.RGBA {
	r := rand.New(rand.NewPCG(seed, 1))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	Clear(img, color.NRGBA{0, 0, 0, 255})
	if dense {
		for i := range img.Pix {
			if i%4 != 3 {
				img.Pix[i] = uint8(r.IntN(256))
			}
		}
		return img
	}
	for range 5 {
		cx, cy, s := r.IntN(w), r.IntN(h), 1+r.IntN(12)
		for y := max(cy-s, 0); y < min(cy+s, h); y++ {
			for x := max(cx-s, 0); x < min(cx+s, w); x++ {
				img.Pix[y*img.Stride+x*4+r.IntN(3)] = uint8(40 + r.IntN(216))
			}
		}
	}
	return img
}

func TestBloomSkipsOnlyWhatAddsNothing(t *testing.T) {
	for _, sz := range [][2]int{{400, 640}, {640, 400}, {203, 117}} {
		for _, dense := range []bool{false, true} {
			for seed := range uint64(6) {
				for _, amount := range []float32{0.05, 0.8, 1.5} {
					src := scene(sz[0], sz[1], dense, seed)
					want, got := image.NewRGBA(src.Bounds()), image.NewRGBA(src.Bounds())
					copy(want.Pix, src.Pix)
					copy(got.Pix, src.Pix)

					var ref Bloom
					ref.glowWith(want, src, amount, 0.03, 2, ref.addUpEverywhere)
					var bl Bloom
					bl.Glow(got, src, amount, 0.03, 2)
					if !bytes.Equal(want.Pix, got.Pix) {
						t.Fatalf("%dx%d dense=%v seed %d amount %v: the skipping bloom differs", sz[0], sz[1], dense, seed, amount)
					}
				}
			}
		}
	}
}

func radialEverywhere(img *image.RGBA, cx, cy, r0, r1 float32, stops []Stop) {
	b := img.Bounds()
	n := min(squares, max(int(r1*r1), 16))
	sq := make([]color.NRGBA, n)
	for k := range sq {
		d := float32(math.Sqrt(float64(k)/float64(n-1))) * r1
		sq[k] = sample(stops, (d-r0)/(r1-r0))
	}
	scale := float32(n-1) / (r1 * r1)
	edge := max(r1-1, 0) * max(r1-1, 0)
	for y := max(int(cy-r1), 0); y < min(int(cy+r1)+1, b.Dy()); y++ {
		for x := max(int(cx-r1), 0); x < min(int(cx+r1)+1, b.Dx()); x++ {
			dx, dy := float32(x)+0.5-cx, float32(y)+0.5-cy
			d2 := dx*dx + dy*dy
			if d2 >= r1*r1 {
				continue
			}
			c := sq[min(int(d2*scale), n-1)]
			if d2 > edge {
				c.A = uint8(float32(c.A) * min(r1-float32(math.Sqrt(float64(d2))), 1))
			}
			put(img.Pix[y*img.Stride+x*4:y*img.Stride+x*4+3], c, true)
		}
	}
}

func TestAdditiveRadialMatchesEveryPixelPut(t *testing.T) {
	r := rand.New(rand.NewPCG(9, 9))
	for i := range 200 {
		w, h := 50+r.IntN(300), 50+r.IntN(300)
		cx, cy := r.Float32()*float32(w)*1.4-float32(w)*0.2, r.Float32()*float32(h)*1.4-float32(h)*0.2
		r1 := 1 + r.Float32()*300
		r0 := r.Float32() * r1 * 0.5
		stops := []Stop{
			{At: 0, C: color.NRGBA{uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256))}},
			{At: r.Float32(), C: color.NRGBA{uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256))}},
			{At: 1, C: color.NRGBA{uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(3))}},
		}
		if i%3 == 0 {
			stops[1].At, stops[1].C.A, stops[2].C.A = 0.3+r.Float32()*0.4, 0, 0
		}
		want := scene(w, h, true, uint64(i))
		got := image.NewRGBA(want.Bounds())
		copy(got.Pix, want.Pix)
		radialEverywhere(want, cx, cy, r0, r1, stops)
		Radial(got, cx, cy, r0, r1, stops, true)
		if !bytes.Equal(want.Pix, got.Pix) {
			t.Fatalf("case %d (%dx%d at %v,%v r %v..%v): additive radial differs", i, w, h, cx, cy, r0, r1)
		}
	}
}

func TestFillMatchesPutPerPixel(t *testing.T) {
	r := rand.New(rand.NewPCG(4, 4))
	for i := range 400 {
		want := scene(64, 1, true, uint64(i))
		got := image.NewRGBA(want.Bounds())
		copy(got.Pix, want.Pix)
		c := color.NRGBA{uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256)), uint8(r.IntN(256))}
		additive := i%2 == 0
		x0, x1 := r.IntN(64), r.IntN(65)
		for x := x0; x < x1; x++ {
			put(want.Pix[x*4:x*4+3], c, additive)
		}
		Span(got, 0, x0, x1, c, additive)
		if !bytes.Equal(want.Pix, got.Pix) {
			t.Fatalf("case %d %v additive=%v %d..%d: fill differs from put", i, c, additive, x0, x1)
		}
	}
}

func boxPerChannel(src, dst []int32, n, lines, r, step, line int) {
	inv := int64(1<<24) / int64(2*r+1)
	for l := range lines {
		base := l * line
		for ch := range 3 {
			var sum int32
			for k := -r; k <= r; k++ {
				sum += src[base+clampi(k, 0, n-1)*step+ch]
			}
			for i := range n {
				dst[base+i*step+ch] = int32(int64(sum) * inv >> 24)
				sum += src[base+min(i+r+1, n-1)*step+ch] - src[base+max(i-r, 0)*step+ch]
			}
		}
	}
}

func TestBoxMatchesOneChannelAtATime(t *testing.T) {
	rn := rand.New(rand.NewPCG(5, 5))
	for _, w := range []int{1, 2, 3, 7, 40, 101} {
		for _, h := range []int{1, 5, 33} {
			for _, r := range []int{1, 2, 5, 20, 60} {
				src := make([]int32, w*h*3)
				for i := range src {
					src[i] = int32(rn.IntN(4081))
				}
				for _, across := range []bool{true, false} {
					want, got := make([]int32, len(src)), make([]int32, len(src))
					if across {
						boxPerChannel(src, want, w, h, r, 3, w*3)
						box(src, got, w, h, r, 3, w*3)
					} else {
						boxPerChannel(src, want, h, w, r, w*3, 3)
						box(src, got, h, w, r, w*3, 3)
					}
					for i := range want {
						if want[i] != got[i] {
							t.Fatalf("%dx%d r %d across=%v: element %d is %d, want %d", w, h, r, across, i, got[i], want[i])
						}
					}
				}
			}
		}
	}
}

func BenchmarkBloomSparseThird(b *testing.B) {
	src := scene(400, 640, false, 3)
	img := image.NewRGBA(src.Bounds())
	var bl Bloom
	for b.Loop() {
		bl.Glow(img, src, 1.2, 0.03, 2)
	}
}
