package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

const boxH = 180

var glyphs = []struct {
	r    rune
	name string
}{
	{'0', "0"}, {'1', "1"}, {'2', "2"}, {'3', "3"}, {'4', "4"},
	{'5', "5"}, {'6', "6"}, {'7', "7"}, {'8', "8"}, {'9', "9"},
	{':', "colon"},
}

func main() {
	var (
		set  = flag.String("set", "", "the set's name, which is its directory under svg/")
		in   = flag.String("in", "", "the font to read")
		bear = flag.Float64(
			"bearing",
			0.08,
			"empty margin down each side, as a fraction of the box",
		)
	)
	flag.Parse()

	if *set == "" || *in == "" {
		flag.Usage()
		os.Exit(2)
	}

	data, err := os.ReadFile(*in)
	if err != nil {
		log.Fatal(err)
	}

	f, err := sfnt.Parse(data)
	if err != nil {
		log.Fatalf("parsing %s: %v", *in, err)
	}
	if err := emit(f, *set, *bear); err != nil {
		log.Fatal(err)
	}
}

func emit(f *sfnt.Font, set string, bearing float64) error {
	var b sfnt.Buffer

	const ppem = 2048
	at := fixed.I(ppem)

	lo, hi := math.Inf(1), math.Inf(-1)
	wide := 0.0

	outlines := make(map[rune][]sfnt.Segment, len(glyphs))

	for _, g := range glyphs {
		i, err := f.GlyphIndex(&b, g.r)
		if err != nil {
			return fmt.Errorf("%q: %w", g.r, err)
		}
		if i == 0 {
			return fmt.Errorf("the font has no %q", g.r)
		}

		segs, err := f.LoadGlyph(&b, i, at, nil)
		if err != nil {
			return fmt.Errorf("%q: %w", g.r, err)
		}

		// sfnt's LoadGlyph segments are only valid until the next call.
		outlines[g.r] = append([]sfnt.Segment(nil), segs...)

		adv, err := f.GlyphAdvance(&b, i, at, 0)
		if err != nil {
			return fmt.Errorf("%q: %w", g.r, err)
		}

		if g.r == ':' {
			continue
		}
		wide = math.Max(wide, f26(adv))

		for _, s := range segs {
			for _, p := range s.Args[:args(s.Op)] {
				lo = math.Min(lo, f26(p.Y))
				hi = math.Max(hi, f26(p.Y))
			}
		}
	}

	if hi <= lo || wide <= 0 {
		return fmt.Errorf("the digits measure nothing")
	}

	scale := boxH / (hi - lo)
	boxW := int(math.Round(wide*scale/(1-2*bearing))) | 1

	dir := filepath.Join("internal", "ui", "numeral", "svg", set)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	for _, g := range glyphs {
		min, max := math.Inf(1), math.Inf(-1)
		for _, s := range outlines[g.r] {
			for _, p := range s.Args[:args(s.Op)] {
				min = math.Min(min, f26(p.X))
				max = math.Max(max, f26(p.X))
			}
		}

		dx := float64(boxW)/2 - (min+max)/2*scale
		dy := -lo * scale

		path := trace(outlines[g.r], scale, dx, dy)
		if path == "" {
			return fmt.Errorf("%q traced to nothing", g.r)
		}

		if err := write(filepath.Join(dir, g.name+".svg"), boxW, path); err != nil {
			return err
		}
	}

	fmt.Printf("wrote %s, %d glyphs in a %dx%d box\n", dir, len(glyphs), boxW, boxH)
	return nil
}

// The Y axis increases downward in both sfnt and SVG.
func trace(segs []sfnt.Segment, scale, dx, dy float64) string {
	var b strings.Builder

	at := func(p fixed.Point26_6) string {
		return fmt.Sprintf("%d %d",
			int(math.Round(f26(p.X)*scale+dx)),
			int(math.Round(f26(p.Y)*scale+dy)))
	}

	for _, s := range segs {
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			fmt.Fprintf(&b, "M%s", at(s.Args[0]))
		case sfnt.SegmentOpLineTo:
			fmt.Fprintf(&b, "L%s", at(s.Args[0]))
		case sfnt.SegmentOpQuadTo:
			fmt.Fprintf(&b, "Q%s %s", at(s.Args[0]), at(s.Args[1]))
		case sfnt.SegmentOpCubeTo:
			fmt.Fprintf(&b, "C%s %s %s", at(s.Args[0]), at(s.Args[1]), at(s.Args[2]))
		}
	}
	return b.String()
}

func args(op sfnt.SegmentOp) int {
	switch op {
	case sfnt.SegmentOpQuadTo:
		return 2
	case sfnt.SegmentOpCubeTo:
		return 3
	}
	return 1
}

func f26(v fixed.Int26_6) float64 { return float64(v) / 64 }

func write(path string, boxW int, d string) error {
	var b strings.Builder

	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, boxW, boxH)
	fmt.Fprintf(&b, "\n  <path d=\"%sZ\"/>\n</svg>\n", d)

	return os.WriteFile(path, []byte(b.String()), 0o644)
}
