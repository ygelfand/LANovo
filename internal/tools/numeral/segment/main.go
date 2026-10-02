// Command segment writes the seven segment numeral set's SVG sources.
//
// Dev only, run by hand, never on the device. The files it writes are checked in and are what the
// converter reads; this exists because a seven segment digit is geometry rather than drawing, and
// eleven hand-written files of the same seven bars would be eleven chances to get a coordinate
// wrong. Nothing here is a derivative of anybody's typeface.
//
//	go run ./internal/tools/numeral/segment
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// The design box, and the bars in it.
//
// Taller than wide, which is what a digit is. The gap is what keeps two bars meeting at a corner
// from reading as one bent bar, and is the whole of what makes this look like lamps behind a mask
// rather than a font.
const (
	boxW, boxH = 100, 180

	thick = 18
	half  = thick / 2
	gap   = 4

	// bearing is the empty margin down each side of the box.
	//
	// Cells are laid out edge to edge, so without it the outer bars of two digits meet and a 11
	// reads as one wide shape. The space belongs in the glyph rather than in the layout: a face
	// spacing the cells itself would have to know this set is bars, and the next set would want a
	// different number.
	bearing = 8
)

// The seven bars, named the way every datasheet names them:
//
//	 aaa
//	f   b
//	 ggg
//	e   c
//	 ddd
const (
	segA = 1 << iota
	segB
	segC
	segD
	segE
	segF
	segG
)

// lit is which bars each digit turns on.
var lit = [10]int{
	0: segA | segB | segC | segD | segE | segF,
	1: segB | segC,
	2: segA | segB | segG | segE | segD,
	3: segA | segB | segG | segC | segD,
	4: segF | segG | segB | segC,
	5: segA | segF | segG | segC | segD,
	6: segA | segF | segG | segE | segC | segD,
	7: segA | segB | segC,
	8: segA | segB | segC | segD | segE | segF | segG,
	9: segA | segB | segC | segD | segF | segG,
}

func main() {
	dir := filepath.Join("internal", "ui", "numeral", "svg", "segment")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}

	for d, on := range lit {
		var paths []string
		for _, s := range []struct {
			bit int
			d   string
		}{
			{segA, across(top())},
			{segB, down(right(), upper())},
			{segC, down(right(), lower())},
			{segD, across(bottom())},
			{segE, down(left(), lower())},
			{segF, down(left(), upper())},
			{segG, across(middle())},
		} {
			if on&s.bit != 0 {
				paths = append(paths, s.d)
			}
		}

		write(filepath.Join(dir, fmt.Sprintf("%d.svg", d)), paths)
	}

	// The colon is the one glyph that is not a bar: two lamps the size of a bar's thickness.
	write(filepath.Join(dir, "colon.svg"), []string{
		dot(boxW/2, boxH/3),
		dot(boxW/2, boxH*2/3),
	})
}

// The bars' positions. Named rather than inlined so the digit table reads as a datasheet.
func top() int    { return half }
func middle() int { return boxH / 2 }
func bottom() int { return boxH - half }
func left() int   { return bearing + half }
func right() int  { return boxW - bearing - half }

// The two halves a vertical bar can occupy, as a pair so it can be handed straight to down.
func upper() [2]int { return [2]int{top() + gap, middle() - gap} }
func lower() [2]int { return [2]int{middle() + gap, bottom() - gap} }

// across is a horizontal bar: a rectangle with its ends brought to a point, which is what makes two
// of them meeting at a corner look mitred rather than overlapped.
func across(cy int) string {
	x0, x1 := left()+gap, right()-gap

	return points([][2]int{
		{x0, cy},
		{x0 + half, cy - half},
		{x1 - half, cy - half},
		{x1, cy},
		{x1 - half, cy + half},
		{x0 + half, cy + half},
	})
}

// down is the same bar stood up.
func down(cx int, span [2]int) string {
	y0, y1 := span[0], span[1]

	return points([][2]int{
		{cx, y0},
		{cx + half, y0 + half},
		{cx + half, y1 - half},
		{cx, y1},
		{cx - half, y1 - half},
		{cx - half, y0 + half},
	})
}

// dot is one lamp of the colon: a square the thickness of a bar. Square rather than pointed like
// the bars, because a diamond at this size reads as a speck of dirt rather than as punctuation.
func dot(cx, cy int) string {
	return points([][2]int{
		{cx - half, cy - half},
		{cx + half, cy - half},
		{cx + half, cy + half},
		{cx - half, cy + half},
	})
}

func points(at [][2]int) string {
	var b strings.Builder
	for i, p := range at {
		if i == 0 {
			fmt.Fprintf(&b, "M%d %d", p[0], p[1])
			continue
		}
		fmt.Fprintf(&b, "L%d %d", p[0], p[1])
	}
	b.WriteString("Z")
	return b.String()
}

func write(path string, paths []string) {
	var b strings.Builder

	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d">`, boxW, boxH)
	b.WriteString("\n")
	for _, d := range paths {
		fmt.Fprintf(&b, "  <path d=\"%s\"/>\n", d)
	}
	b.WriteString("</svg>\n")

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		log.Fatal(err)
	}
}
