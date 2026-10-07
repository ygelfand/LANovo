package display

import "testing"

func memPanel(w, h int, rot Orientation) *Panel {
	p := &Panel{fbW: w, fbH: h, stride: w * 4}
	p.mem = make([]byte, w*h*4)
	p.Turn(rot)
	return p
}

func TestDrawRGBAScalesAndRotates(t *testing.T) {
	src := []byte{
		10, 0, 0, 255, 20, 0, 0, 255,
		30, 0, 0, 255, 40, 0, 0, 255,
	}
	for _, rot := range []Orientation{Rotate0, Rotate90, Rotate180, Rotate270} {
		p := memPanel(8, 6, rot)
		p.DrawRGBA(1, 1, src, 8, 2, 2, 2, Rect{})
		for _, c := range []struct {
			vx, vy int
			want   byte
		}{{1, 1, 10}, {2, 2, 10}, {3, 1, 20}, {1, 3, 30}, {4, 4, 40}} {
			x, y := rot.Project(p.fbW, p.fbH, c.vx, c.vy)
			if got := p.mem[y*p.stride+x*4]; got != c.want {
				t.Errorf("%v: (%d,%d) = %d, want %d", rot, c.vx, c.vy, got, c.want)
			}
		}
		x, y := rot.Project(p.fbW, p.fbH, 0, 0)
		if got := p.mem[y*p.stride+x*4]; got != 0 {
			t.Errorf("%v: outside the image painted %d", rot, got)
		}
	}
}

func TestDrawRGBAKeepsToTheClip(t *testing.T) {
	src := make([]byte, 4*4*4)
	for i := range src {
		src[i] = 99
	}
	p := memPanel(8, 8, Rotate0)
	p.DrawRGBA(0, 0, src, 16, 4, 4, 2, Rect{X: 2, Y: 2, W: 2, H: 2})
	if p.mem[1*p.stride+1*4] != 0 || p.mem[4*p.stride+4*4] != 0 {
		t.Error("painted outside the clip")
	}
	if p.mem[2*p.stride+2*4] != 99 {
		t.Error("nothing painted inside the clip")
	}
}

func TestDrawRGBAMatchesAPixelAtATime(t *testing.T) {
	const w, h = 7, 5
	src := make([]byte, w*h*4)
	for i := range src {
		src[i] = byte(i*7 + 1)
	}
	for _, rot := range []Orientation{Rotate0, Rotate90, Rotate180, Rotate270} {
		for _, scale := range []int{1, 2, 3} {
			for _, at := range [][2]int{{0, 0}, {3, 2}, {-4, -3}, {9, 11}} {
				for _, clip := range []Rect{{}, {X: 2, Y: 3, W: 9, H: 6}, {X: 10, Y: 1, W: 3, H: 20}} {
					got := memPanel(18, 22, rot)
					want := memPanel(18, 22, rot)
					got.DrawRGBA(at[0], at[1], src, w*4, w, h, scale, clip)
					for vy := range want.Height {
						for vx := range want.Width {
							sx, sy := vx-at[0], vy-at[1]
							if sx < 0 || sy < 0 || sx >= w*scale || sy >= h*scale {
								continue
							}
							if clip.W > 0 &&
								(vx < clip.X || vx >= clip.X+clip.W || vy < clip.Y || vy >= clip.Y+clip.H) {
								continue
							}
							s := (sy/scale)*w*4 + (sx/scale)*4
							fx, fy := rot.Project(want.fbW, want.fbH, vx, vy)
							copy(
								want.mem[fy*want.stride+fx*4:],
								[]byte{src[s], src[s+1], src[s+2], 0xff},
							)
						}
					}
					if string(got.mem) != string(want.mem) {
						t.Fatalf(
							"%v scale %d at %v clip %v: differs from a pixel at a time",
							rot,
							scale,
							at,
							clip,
						)
					}
				}
			}
		}
	}
}

func BenchmarkDrawRGBAFullThird(b *testing.B) {
	for _, rot := range []Orientation{Rotate0, Rotate90} {
		b.Run(rot.String(), func(b *testing.B) {
			p := memPanel(1200, 1920, rot)
			src := make([]byte, 640*640*4)
			for b.Loop() {
				p.DrawRGBA(0, 0, src, p.Width/3*4, p.Width/3, p.Height/3, 3, Rect{})
			}
		})
	}
}
