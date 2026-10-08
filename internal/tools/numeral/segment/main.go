package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	boxW, boxH = 100, 180

	thick = 18
	half  = thick / 2
	gap   = 4

	bearing = 8
)

// Segments a to g, named as every datasheet names them.
const (
	segA = 1 << iota
	segB
	segC
	segD
	segE
	segF
	segG
)

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

	write(filepath.Join(dir, "colon.svg"), []string{
		dot(boxW/2, boxH/3),
		dot(boxW/2, boxH*2/3),
	})
}

func top() int    { return half }
func middle() int { return boxH / 2 }
func bottom() int { return boxH - half }
func left() int   { return bearing + half }
func right() int  { return boxW - bearing - half }

func upper() [2]int { return [2]int{top() + gap, middle() - gap} }
func lower() [2]int { return [2]int{middle() + gap, bottom() - gap} }

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
