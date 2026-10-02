package display

import "github.com/ygelfand/LANovo/internal/lib/surface"

type Rect struct{ X, Y, W, H int }

func (o Orientation) point(fbW, fbH int, vx, vy float32) (float32, float32) {
	switch o {
	case Rotate90:
		return vy, float32(fbH) - vx
	case Rotate180:
		return float32(fbW) - vx, float32(fbH) - vy
	case Rotate270:
		return float32(fbW) - vy, vx
	}
	return vx, vy
}

func (o Orientation) Place(fbW, fbH int, r Rect, w, h int) (x, y float32, m surface.Matrix, at Rect) {
	x0, y0 := o.point(fbW, fbH, float32(r.X), float32(r.Y))
	x1, y1 := o.point(fbW, fbH, float32(r.X+r.W), float32(r.Y))
	x2, y2 := o.point(fbW, fbH, float32(r.X), float32(r.Y+r.H))
	m = surface.Matrix{
		DsDx: (x1 - x0) / float32(w),
		DtDx: (y1 - y0) / float32(w),
		DtDy: (x2 - x0) / float32(h),
		DsDy: (y2 - y0) / float32(h),
	}
	lx, ly := min(x0, x1, x2), min(y0, y1, y2)
	hx, hy := max(x0, x1, x2, x1+x2-x0), max(y0, y1, y2, y1+y2-y0)
	return x0, y0, m, Rect{X: int(lx), Y: int(ly), W: int(hx - lx), H: int(hy - ly)}
}
