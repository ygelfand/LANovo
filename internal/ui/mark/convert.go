// Package mark turns SVG icons into the IconVG the panel draws.
//
// The icon set the binary carries is Material Design, which arrives already converted. This is for
// everything else: any icon set whose glyphs are filled paths can be taken from, a file at a time,
// without depending on the set or shipping a rasterizer for it.
//
// Filled paths only. Sets drawn as strokes with no fill — Lucide and Feather among them — come out
// empty, because nothing here widens a stroke into an outline.
//
// The conversion is adapted from golang.org/x/exp/shiny/materialdesign/icons/gen.go, which is
// Copyright 2016 The Go Authors and BSD licensed.
package mark

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/exp/shiny/iconvg"
	"golang.org/x/image/math/f32"

	"github.com/ygelfand/LANovo/internal/ui"
)

// size is the width and height in IconVG's own space, whatever the SVG was drawn at.
const size = 48

// svg is as much of the format as a single-color icon uses.
type svg struct {
	ViewBox string   `xml:"viewBox,attr"`
	Paths   []path   `xml:"path"`
	Circles []circle `xml:"circle"`
}

type path struct {
	D string `xml:"d,attr"`
}

// IconVG has no circles, so they become a pair of half arcs.
type circle struct {
	Cx float32 `xml:"cx,attr"`
	Cy float32 `xml:"cy,attr"`
	R  float32 `xml:"r,attr"`
}

// Convert turns one SVG into an icon. The viewBox may be any square; what comes out is scaled to
// IconVG's space so icons from different sets are the same size beside each other.
//
// A box that is not square is centered in one that is, because an icon is drawn into a square and
// anything else would be stretched to fill it.
func Convert(data []byte) (ui.Icon, error) { return convert(data, false) }

// Shaped is Convert for artwork whose box is not square and is meant not to be.
//
// A numeral is the case: it is drawn into a tall box, laid out by that box's proportions, and drawn
// filling whatever it is given. Squaring it would declare half a box of margin that the layout then
// has to know about and subtract, and would carry that margin in every mask.
func Shaped(data []byte) (ui.Icon, error) { return convert(data, true) }

func convert(data []byte, shaped bool) (ui.Icon, error) {
	var in svg
	if err := xml.Unmarshal(data, &in); err != nil {
		return nil, fmt.Errorf("mark: reading the svg: %w", err)
	}

	side, extent, offset, err := box(in.ViewBox)
	if err != nil {
		return nil, err
	}
	if len(in.Paths) == 0 && len(in.Circles) == 0 {
		return nil, fmt.Errorf("mark: the svg has no filled paths")
	}

	// Square unless the caller says the shape is the point, in which case the declared box is the
	// drawing's own. Coordinates are the same either way: only what the rasterizer is told it is
	// filling changes.
	if !shaped {
		extent = f32.Vec2{size, size}
	}

	var enc iconvg.Encoder
	enc.Reset(iconvg.Metadata{
		ViewBox: iconvg.Rectangle{
			Min: f32.Vec2{-extent[0] / 2, -extent[1] / 2},
			Max: f32.Vec2{+extent[0] / 2, +extent[1] / 2},
		},
		Palette: iconvg.DefaultPalette,
	})

	started := false
	for _, p := range in.Paths {
		if p.D == "" {
			continue
		}
		if err := draw(&enc, p.D, side, offset, &started); err != nil {
			return nil, err
		}
	}

	for _, c := range in.Circles {
		cx := c.Cx*size/side - size/2 - offset[0]
		cy := c.Cy*size/side - size/2 - offset[1]
		r := c.R * size / side

		if !started {
			started = true
			enc.StartPath(0, cx-r, cy)
		} else {
			enc.ClosePathAbsMoveTo(cx-r, cy)
		}

		// Two half turns: one arc of a whole turn starts and ends in the same place, which is
		// degenerate.
		enc.RelArcTo(r, r, 0, false, true, +2*r, 0)
		enc.RelArcTo(r, r, 0, false, true, -2*r, 0)
	}

	if !started {
		return nil, fmt.Errorf("mark: the svg drew nothing")
	}
	enc.ClosePathEndPath()

	out, err := enc.Bytes()
	if err != nil {
		return nil, fmt.Errorf("mark: encoding: %w", err)
	}
	return ui.Icon(out), nil
}

// box reads the viewBox, which says what the coordinates in the path data are relative to.
//
// side is what the coordinates are scaled by, taken from the longer edge so a drawing of any shape
// comes out the same size beside others. extent is that shape in IconVG's own space, which is the
// box the drawing is declared to occupy. offset centers each axis on the origin.
func box(viewBox string) (side float32, extent, offset f32.Vec2, err error) {
	if viewBox == "" {
		// The overwhelmingly common one, and what a file with no box is almost certainly drawn in.
		return 24, f32.Vec2{size, size}, f32.Vec2{}, nil
	}

	fields := strings.Fields(strings.ReplaceAll(viewBox, ",", " "))
	if len(fields) != 4 {
		return 0, extent, offset, fmt.Errorf("mark: viewBox %q is not four numbers", viewBox)
	}

	var at [4]float32
	for i, f := range fields {
		v, err := strconv.ParseFloat(f, 32)
		if err != nil {
			return 0, extent, offset, fmt.Errorf("mark: viewBox %q: %w", viewBox, err)
		}
		at[i] = float32(v)
	}

	w, h := at[2], at[3]
	if w <= 0 || h <= 0 {
		return 0, extent, offset, fmt.Errorf("mark: viewBox %q has no area", viewBox)
	}

	side = max(w, h)
	extent = f32.Vec2{w * size / side, h * size / side}
	offset = f32.Vec2{
		(at[0] + (w-side)/2) * size / side,
		(at[1] + (h-side)/2) * size / side,
	}
	return side, extent, offset, nil
}

// draw walks one path's data, emitting it as IconVG.
func draw(enc *iconvg.Encoder, data string, side float32, offset f32.Vec2, started *bool) error {
	data = strings.TrimSuffix(strings.TrimSpace(data), "z")
	r := strings.NewReader(strings.ReplaceAll(data, ",", " "))

	var args [6]float32
	op, relative := byte(0), false

	for {
		b, err := r.ReadByte()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch {
		case b == ' ' || b == '\n' || b == '\t':
			continue
		case 'A' <= b && b <= 'Z':
			op, relative = b, false
		case 'a' <= b && b <= 'z':
			op, relative = b, true
		default:
			r.UnreadByte()
		}

		n := 0
		switch op {
		case 'H', 'h', 'V', 'v':
			n = 1
		case 'L', 'l', 'T', 't', 'M', 'm':
			n = 2
		case 'Q', 'q', 'S', 's':
			n = 4
		case 'C', 'c':
			n = 6
		case 'Z', 'z':
		default:
			return fmt.Errorf("mark: unknown path command %q", string(b))
		}

		scan(&args, r, n)
		normalize(&args, n, op, side, offset, relative)

		switch op {
		case 'L':
			enc.AbsLineTo(args[0], args[1])
		case 'l':
			enc.RelLineTo(args[0], args[1])
		case 'T':
			enc.AbsSmoothQuadTo(args[0], args[1])
		case 't':
			enc.RelSmoothQuadTo(args[0], args[1])
		case 'Q':
			enc.AbsQuadTo(args[0], args[1], args[2], args[3])
		case 'q':
			enc.RelQuadTo(args[0], args[1], args[2], args[3])
		case 'S':
			enc.AbsSmoothCubeTo(args[0], args[1], args[2], args[3])
		case 's':
			enc.RelSmoothCubeTo(args[0], args[1], args[2], args[3])
		case 'C':
			enc.AbsCubeTo(args[0], args[1], args[2], args[3], args[4], args[5])
		case 'c':
			enc.RelCubeTo(args[0], args[1], args[2], args[3], args[4], args[5])
		case 'H':
			enc.AbsHLineTo(args[0])
		case 'h':
			enc.RelHLineTo(args[0])
		case 'V':
			enc.AbsVLineTo(args[0])
		case 'v':
			enc.RelVLineTo(args[0])
		case 'M':
			if !*started {
				*started = true
				enc.StartPath(0, args[0], args[1])
			} else {
				enc.ClosePathAbsMoveTo(args[0], args[1])
			}
		case 'm':
			if !*started {
				*started = true
				enc.StartPath(0, args[0], args[1])
			} else {
				enc.ClosePathRelMoveTo(args[0], args[1])
			}
		}
	}
}

func scan(args *[6]float32, r *strings.Reader, n int) {
	for i := range n {
		for {
			if b, _ := r.ReadByte(); b != ' ' {
				r.UnreadByte()
				break
			}
		}
		fmt.Fscanf(r, "%f", &args[i])
	}
}

// normalize scales path coordinates into IconVG's space, which is centered on the origin where an
// SVG's is not.
func normalize(args *[6]float32, n int, op byte, side float32, offset f32.Vec2, relative bool) {
	for i := range n {
		args[i] *= size / side
		if relative {
			continue
		}
		args[i] -= size / 2

		switch {
		case n != 1:
			args[i] -= offset[i&0x01]
		case op == 'H':
			args[i] -= offset[0]
		case op == 'V':
			args[i] -= offset[1]
		}
	}
}

// Must is Convert for an icon compiled in, where a file that will not convert is a mistake in the
// build rather than something to handle.
func Must(data []byte) ui.Icon {
	icon, err := Convert(data)
	if err != nil {
		panic(err)
	}
	return icon
}
