package main

import (
	"fmt"

	"golang.org/x/exp/shiny/iconvg"
	"golang.org/x/image/math/f32"
)

const (
	side = 48
	half = side / 2
)

type box struct {
	x, y, w, h float32
}

func (b box) scale() float32 { return side / max(b.w, b.h) }

func (b box) at(x, y float32) (float32, float32) {
	s := b.scale()
	return (x-b.x)*s - b.w*s/2, (y-b.y)*s - b.h*s/2
}

func (b box) by(dx, dy float32) (float32, float32) {
	s := b.scale()
	return dx * s, dy * s
}

func convert(enc *iconvg.Encoder, b box, d string) error {
	p := &scanner{s: d}

	var op byte
	var started bool

	var cx, cy float32

	var sx, sy float32

	for !p.done() {
		if c := p.command(); c != 0 {
			op = c
		} else if op == 'M' {
			// SVG: coordinates repeated after a moveto are linetos.
			op = 'L'
		} else if op == 'm' {
			op = 'l'
		}

		if op == 0 {
			return fmt.Errorf("icon: path data starts with a number")
		}

		abs := op >= 'A' && op <= 'Z'

		num := func(n int) ([]float32, error) {
			out := make([]float32, n)
			for i := range out {
				v, err := p.number()
				if err != nil {
					return nil, err
				}
				out[i] = v
			}
			return out, nil
		}

		switch op {
		case 'M', 'm':
			a, err := num(2)
			if err != nil {
				return err
			}

			if abs {
				cx, cy = a[0], a[1]
			} else {
				cx, cy = cx+a[0], cy+a[1]
			}
			sx, sy = cx, cy

			x, y := b.at(cx, cy)
			if !started {
				enc.StartPath(0, x, y)
				started = true
			} else {
				enc.ClosePathAbsMoveTo(x, y)
			}

		case 'Z', 'z':
			cx, cy = sx, sy

		case 'L', 'l':
			a, err := num(2)
			if err != nil {
				return err
			}
			if abs {
				cx, cy = a[0], a[1]
				x, y := b.at(cx, cy)
				enc.AbsLineTo(x, y)
			} else {
				cx, cy = cx+a[0], cy+a[1]
				dx, dy := b.by(a[0], a[1])
				enc.RelLineTo(dx, dy)
			}

		case 'H', 'h':
			a, err := num(1)
			if err != nil {
				return err
			}
			if abs {
				cx = a[0]
				x, _ := b.at(cx, cy)
				enc.AbsHLineTo(x)
			} else {
				cx += a[0]
				dx, _ := b.by(a[0], 0)
				enc.RelHLineTo(dx)
			}

		case 'V', 'v':
			a, err := num(1)
			if err != nil {
				return err
			}
			if abs {
				cy = a[0]
				_, y := b.at(cx, cy)
				enc.AbsVLineTo(y)
			} else {
				cy += a[0]
				_, dy := b.by(0, a[0])
				enc.RelVLineTo(dy)
			}

		case 'C', 'c':
			a, err := num(6)
			if err != nil {
				return err
			}
			if abs {
				x1, y1 := b.at(a[0], a[1])
				x2, y2 := b.at(a[2], a[3])
				x, y := b.at(a[4], a[5])
				enc.AbsCubeTo(x1, y1, x2, y2, x, y)
				cx, cy = a[4], a[5]
			} else {
				x1, y1 := b.by(a[0], a[1])
				x2, y2 := b.by(a[2], a[3])
				x, y := b.by(a[4], a[5])
				enc.RelCubeTo(x1, y1, x2, y2, x, y)
				cx, cy = cx+a[4], cy+a[5]
			}

		case 'S', 's':
			a, err := num(4)
			if err != nil {
				return err
			}
			if abs {
				x2, y2 := b.at(a[0], a[1])
				x, y := b.at(a[2], a[3])
				enc.AbsSmoothCubeTo(x2, y2, x, y)
				cx, cy = a[2], a[3]
			} else {
				x2, y2 := b.by(a[0], a[1])
				x, y := b.by(a[2], a[3])
				enc.RelSmoothCubeTo(x2, y2, x, y)
				cx, cy = cx+a[2], cy+a[3]
			}

		case 'Q', 'q':
			a, err := num(4)
			if err != nil {
				return err
			}
			if abs {
				x1, y1 := b.at(a[0], a[1])
				x, y := b.at(a[2], a[3])
				enc.AbsQuadTo(x1, y1, x, y)
				cx, cy = a[2], a[3]
			} else {
				x1, y1 := b.by(a[0], a[1])
				x, y := b.by(a[2], a[3])
				enc.RelQuadTo(x1, y1, x, y)
				cx, cy = cx+a[2], cy+a[3]
			}

		case 'T', 't':
			a, err := num(2)
			if err != nil {
				return err
			}
			if abs {
				x, y := b.at(a[0], a[1])
				enc.AbsSmoothQuadTo(x, y)
				cx, cy = a[0], a[1]
			} else {
				dx, dy := b.by(a[0], a[1])
				enc.RelSmoothQuadTo(dx, dy)
				cx, cy = cx+a[0], cy+a[1]
			}

		case 'A', 'a':
			rx, err := p.number()
			if err != nil {
				return err
			}
			ry, err := p.number()
			if err != nil {
				return err
			}
			rot, err := p.number()
			if err != nil {
				return err
			}
			large, err := p.flag()
			if err != nil {
				return err
			}
			sweep, err := p.flag()
			if err != nil {
				return err
			}
			ex, err := p.number()
			if err != nil {
				return err
			}
			ey, err := p.number()
			if err != nil {
				return err
			}

			sr := b.scale()

			if abs {
				x, y := b.at(ex, ey)
				enc.AbsArcTo(rx*sr, ry*sr, rot, large, sweep, x, y)
				cx, cy = ex, ey
			} else {
				dx, dy := b.by(ex, ey)
				enc.RelArcTo(rx*sr, ry*sr, rot, large, sweep, dx, dy)
				cx, cy = cx+ex, cy+ey
			}

		default:
			return fmt.Errorf("icon: %q is not a path command", string(op))
		}
	}

	if !started {
		return fmt.Errorf("icon: the path drew nothing")
	}
	return nil
}

func start() *iconvg.Encoder {
	var enc iconvg.Encoder
	enc.Reset(iconvg.Metadata{
		ViewBox: iconvg.Rectangle{
			Min: f32.Vec2{-half, -half},
			Max: f32.Vec2{+half, +half},
		},
		Palette: iconvg.DefaultPalette,
	})
	return &enc
}
