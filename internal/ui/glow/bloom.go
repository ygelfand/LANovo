package glow

import "image"

type Bloom struct {
	octaves []octave

	xi0, xi1 []int32
	wx       []int32
	forW     int
	rowA     []int32
	rowB     []int32
	lo, hi   []int32
}

type octave struct {
	w, h    int
	px, tmp []int32
}

// Apply adds a blurred copy of img back onto it, radius as a fraction of the height, the way the
// reference's Renderer.bloom does: 1/4 size first, then 1/8, each weaker.
func (bl *Bloom) Apply(img *image.RGBA, amount, radius float32, passes int) {
	bl.Glow(img, img, amount, radius, passes)
}

// Glow adds a blurred copy of src onto img, for a scene where only part of it gives off light.
func (bl *Bloom) Glow(img, src *image.RGBA, amount, radius float32, passes int) {
	bl.glowWith(img, src, amount, radius, passes, bl.addUp)
}

func (bl *Bloom) glowWith(img, src *image.RGBA, amount, radius float32, passes int, add func(*image.RGBA, *octave, int32)) {
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()
	passes = max(passes, 1)

	first := bl.octave(0, max(W/4, 2), max(H/4, 2))
	first.fromImage(src)
	for p := 1; p < passes; p++ {
		prev := &bl.octaves[p-1]
		bl.octave(p, max(prev.w/2, 2), max(prev.h/2, 2)).fromOctave(prev)
	}

	for p := range passes {
		o := &bl.octaves[p]
		r := max(int(radius*float32(H)*float32(1+p)/float32(int(4)<<p)+0.5), 1)
		o.blur(r)
		o.blur(r)
	}

	k := int32(amount * 256)
	for p := passes - 1; p > 0; p-- {
		weight := int32(float32(k) / (1 + float32(p)*0.6))
		bl.octaves[p-1].addFrom(&bl.octaves[p], weight, k)
	}
	add(img, first, k)
}

func (bl *Bloom) octave(p, w, h int) *octave {
	for len(bl.octaves) <= p {
		bl.octaves = append(bl.octaves, octave{})
	}
	o := &bl.octaves[p]
	if o.w != w || o.h != h {
		o.w, o.h = w, h
		o.px, o.tmp = make([]int32, w*h*3), make([]int32, w*h*3)
	}
	return o
}

func (o *octave) fromImage(img *image.RGBA) {
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()
	clear(o.px)
	for y := range min(H, o.h*4) {
		row := img.Pix[y*img.Stride:]
		out := o.px[(y/4)*o.w*3:]
		for x := range min(W, o.w*4) {
			i, j := x*4, (x/4)*3
			out[j] += int32(row[i])
			out[j+1] += int32(row[i+1])
			out[j+2] += int32(row[i+2])
		}
	}
}

func (o *octave) fromOctave(src *octave) {
	for y := range o.h {
		for x := range o.w {
			i := (y*o.w + x) * 3
			for ch := range 3 {
				a := src.px[((2*y)*src.w+2*x)*3+ch] + src.px[((2*y)*src.w+min(2*x+1, src.w-1))*3+ch]
				c := src.px[(min(2*y+1, src.h-1)*src.w+2*x)*3+ch] + src.px[(min(2*y+1, src.h-1)*src.w+min(2*x+1, src.w-1))*3+ch]
				o.px[i+ch] = (a + c) / 4
			}
		}
	}
}

func (o *octave) blur(r int) {
	box(o.px, o.tmp, o.w, o.h, r, 3, o.w*3)
	box(o.tmp, o.px, o.h, o.w, r, o.w*3, 3)
}

func box(src, dst []int32, n, lines, r, step, line int) {
	inv := int64(1<<24) / int64(2*r+1)
	for l := range lines {
		base := l * line
		var s0, s1, s2 int32
		for k := -r; k <= r; k++ {
			j := base + clampi(k, 0, n-1)*step
			s0, s1, s2 = s0+src[j], s1+src[j+1], s2+src[j+2]
		}
		i := 0
		for ; i < n && (i < r || i+r+1 > n-1); i++ {
			d := dst[base+i*step : base+i*step+3 : base+i*step+3]
			d[0], d[1], d[2] = int32(int64(s0)*inv>>24), int32(int64(s1)*inv>>24), int32(int64(s2)*inv>>24)
			a, b := base+min(i+r+1, n-1)*step, base+max(i-r, 0)*step
			s0, s1, s2 = s0+src[a]-src[b], s1+src[a+1]-src[b+1], s2+src[a+2]-src[b+2]
		}
		for ; i+r+1 <= n-1; i++ {
			d := dst[base+i*step : base+i*step+3 : base+i*step+3]
			d[0], d[1], d[2] = int32(int64(s0)*inv>>24), int32(int64(s1)*inv>>24), int32(int64(s2)*inv>>24)
			a := src[base+(i+r+1)*step : base+(i+r+1)*step+3 : base+(i+r+1)*step+3]
			b := src[base+(i-r)*step : base+(i-r)*step+3 : base+(i-r)*step+3]
			s0, s1, s2 = s0+a[0]-b[0], s1+a[1]-b[1], s2+a[2]-b[2]
		}
		for ; i < n; i++ {
			d := dst[base+i*step : base+i*step+3 : base+i*step+3]
			d[0], d[1], d[2] = int32(int64(s0)*inv>>24), int32(int64(s1)*inv>>24), int32(int64(s2)*inv>>24)
			a, b := base+min(i+r+1, n-1)*step, base+max(i-r, 0)*step
			s0, s1, s2 = s0+src[a]-src[b], s1+src[a+1]-src[b+1], s2+src[a+2]-src[b+2]
		}
	}
}

func (o *octave) addFrom(src *octave, weight, base int32) {
	k := int32(int64(weight) << 8 / int64(max(base, 1)))
	for y := range o.h {
		sy := min(y>>1, src.h-1)
		for x := range o.w {
			sx := min(x>>1, src.w-1)
			i, j := (y*o.w+x)*3, (sy*src.w+sx)*3
			for ch := range 3 {
				o.px[i+ch] += src.px[j+ch] * k >> 8
			}
		}
	}
}

func (bl *Bloom) columns(W int, o *octave) {
	if bl.forW == W && len(bl.xi0) == W {
		return
	}
	bl.forW = W
	bl.xi0, bl.xi1, bl.wx = make([]int32, W), make([]int32, W), make([]int32, W)
	bl.rowA, bl.rowB = make([]int32, W*3), make([]int32, W*3)
	sx := float32(o.w) / float32(W)
	for x := range W {
		fx := (float32(x)+0.5)*sx - 0.5
		x0 := clampi(int(fx), 0, o.w-1)
		bl.xi0[x], bl.xi1[x] = int32(x0*3), int32(min(x0+1, o.w-1)*3)
		bl.wx[x] = int32(min(max(fx-float32(x0), 0), 1) * 256)
	}
}

func (bl *Bloom) spread(dst []int32, o *octave, y int, k int32) {
	src := o.px[y*o.w*3:]
	for x := range len(bl.xi0) {
		a, c, w := bl.xi0[x], bl.xi1[x], bl.wx[x]
		d := dst[x*3 : x*3+3 : x*3+3]
		d[0] = (src[a]*(256-w) + src[c]*w) >> 8 * k >> 8
		d[1] = (src[a+1]*(256-w) + src[c+1]*w) >> 8 * k >> 8
		d[2] = (src[a+2]*(256-w) + src[c+2]*w) >> 8 * k >> 8
	}
}

func (bl *Bloom) lit(o *octave, k int32) {
	if len(bl.lo) != o.h {
		bl.lo, bl.hi = make([]int32, o.h), make([]int32, o.h)
	}
	floor := int32(int64(16)<<8/int64(max(k, 1))) - 1
	for y := range o.h {
		row := o.px[y*o.w*3 : (y+1)*o.w*3]
		lo, hi := int32(o.w), int32(0)
		for x := range o.w {
			if max(row[x*3], row[x*3+1], row[x*3+2]) > floor {
				lo = min(lo, int32(x))
				hi = int32(x + 1)
			}
		}
		bl.lo[y], bl.hi[y] = lo, hi
	}
}

func (bl *Bloom) span(o *octave, y0, y1, W int) (int, int) {
	lo, hi := min(bl.lo[y0], bl.lo[y1]), max(bl.hi[y0], bl.hi[y1])
	if lo >= hi {
		return 0, 0
	}
	sx := float32(W) / float32(o.w)
	return max(int(float32(lo-1)*sx)-2, 0), min(int(float32(hi+1)*sx)+2, W)
}

func (bl *Bloom) addUp(img *image.RGBA, o *octave, k int32) {
	b := img.Bounds()
	W, H := b.Dx(), b.Dy()
	bl.columns(W, o)
	bl.lit(o, k)

	sy := float32(o.h) / float32(H)
	lastA, lastB := -1, -1
	for y := range H {
		fy := (float32(y)+0.5)*sy - 0.5
		y0 := clampi(int(fy), 0, o.h-1)
		y1 := min(y0+1, o.h-1)
		x0, x1 := bl.span(o, y0, y1, W)
		if x0 >= x1 {
			continue
		}
		wy := int32(min(max(fy-float32(y0), 0), 1) * 256)
		if y0 != lastA {
			if y0 == lastB {
				bl.rowA, bl.rowB = bl.rowB, bl.rowA
			} else {
				bl.spread(bl.rowA, o, y0, k)
			}
			lastA, lastB = y0, -1
		}
		if y1 != lastB {
			bl.spread(bl.rowB, o, y1, k)
			lastB = y1
		}
		row := img.Pix[y*img.Stride : y*img.Stride+W*4]
		ra, rb := bl.rowA, bl.rowB
		w0 := 256 - wy
		for x := x0; x < x1; x++ {
			p := row[x*4 : x*4+3 : x*4+3]
			a := ra[x*3 : x*3+3 : x*3+3]
			c := rb[x*3 : x*3+3 : x*3+3]
			p[0] = uint8(min(int32(p[0])+(a[0]*w0+c[0]*wy)>>12, 255))
			p[1] = uint8(min(int32(p[1])+(a[1]*w0+c[1]*wy)>>12, 255))
			p[2] = uint8(min(int32(p[2])+(a[2]*w0+c[2]*wy)>>12, 255))
		}
	}
}
