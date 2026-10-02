// Command font pulls a numeral set's SVG sources straight out of a TTF or OTF.
//
// Dev only, run by hand, never on the device. The files it writes are checked in and are what the
// converter reads.
//
//	go run ./internal/tools/numeral/font -set round -in ~/Downloads/Whatever-Bold.ttf
//
// Nothing is hand traced. x/image/font/sfnt hands back a glyph's outline as segments, which is the
// same curve data the SVG wants, so this is a transcription rather than a drawing. The alternative
// was fonttools as a build dependency; sfnt is already here because the text drawing sits on it.
//
// The digits are put in one box and given one advance, which fonts nearly always already do for
// figures and which this enforces rather than assumes: tabular is the property the whole package
// exists for, and a face with proportional figures would quietly take it away.
//
// Licensing is the caller's problem and a real one. A converted outline is still a derivative, so
// the source face's licence belongs in svg/<set>/LICENCE beside what came out of it. Only convert
// what the licence allows.
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

// The box the digits are normalized into. Tall, and a round number so the path data reads.
const boxH = 180

// glyphs are the eleven a set has to have: the runes, and what each file is called.
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
		bear = flag.Float64("bearing", 0.08, "empty margin down each side, as a fraction of the box")
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

	// Everything is measured at one size and scaled from it. The size is large so the rounding to
	// whole path units is well under a pixel at any size the panel draws.
	const ppem = 2048
	at := fixed.I(ppem)

	// The box comes from the digits themselves rather than from the font's metrics. A font's ascent
	// includes room for accents no numeral uses, and sizing to it leaves the clock sitting in a
	// band of nothing with no way to tell how much.
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

		// Copied, because what comes back points into the buffer and is only good until the next
		// call. Keeping the slice leaves every glyph holding whichever one was loaded last.
		outlines[g.r] = append([]sfnt.Segment(nil), segs...)

		adv, err := f.GlyphAdvance(&b, i, at, 0)
		if err != nil {
			return fmt.Errorf("%q: %w", g.r, err)
		}

		// The colon is excluded from both. It is short, so it would shrink the box; it is narrow, so
		// it would narrow the advance. Neither is what a row of digits is laid out on.
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

	// Scale so the digits fill the box's height, then put the advance in the middle of a box widened
	// by the bearing, so two digits side by side do not touch.
	scale := boxH / (hi - lo)
	boxW := int(math.Round(wide*scale/(1-2*bearing))) | 1

	dir := filepath.Join("internal", "ui", "numeral", "svg", set)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	for _, g := range glyphs {
		// x is centered on the glyph's own advance so a narrow 1 sits in the middle of its cell
		// rather than against the left of it.
		var min, max float64 = math.Inf(1), math.Inf(-1)
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

// trace turns a glyph's segments into SVG path data.
//
// The Y axis already increases downward in both, so nothing is flipped. Coordinates are rounded to
// whole units: at this scale that is far under a pixel, and it keeps the checked-in files readable
// and their diffs meaningful.
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

// args is how many of a segment's three points it actually uses.
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
