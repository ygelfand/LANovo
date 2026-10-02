package ui

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

var (
	black = theme.Color{}
	white = theme.Color{R: 255, G: 255, B: 255}
	red   = theme.Color{R: 255}
)

func TestRectInset(t *testing.T) {
	got := Rect{X: 10, Y: 20, W: 100, H: 50}.Inset(5)
	want := Rect{X: 15, Y: 25, W: 90, H: 40}

	if got != want {
		t.Errorf("Inset(5) = %+v, want %+v", got, want)
	}
}

func TestRectCenter(t *testing.T) {
	x, y := Rect{X: 10, Y: 20, W: 100, H: 50}.Center()
	if x != 60 || y != 45 {
		t.Errorf("Center() = %d,%d, want 60,45", x, y)
	}
}

// Contains is what a touch uses to find what it hit, so the edges have to be right: the top-left
// is inside and the bottom-right is not.
func TestRectContains(t *testing.T) {
	r := Rect{X: 10, Y: 10, W: 20, H: 20}

	tests := []struct {
		name string
		x, y int
		want bool
	}{
		{"the top-left corner is inside", 10, 10, true},
		{"the middle", 20, 20, true},
		{"the last pixel inside", 29, 29, true},
		{"one past the right edge", 30, 20, false},
		{"one past the bottom edge", 20, 30, false},
		{"above", 20, 9, false},
		{"left", 9, 20, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.Contains(tt.x, tt.y); got != tt.want {
				t.Errorf("Contains(%d, %d) = %v, want %v", tt.x, tt.y, got, tt.want)
			}
		})
	}
}

func TestFillRect(t *testing.T) {
	img := NewImage(20, 20, black)
	FillRect(img, Rect{X: 5, Y: 5, W: 10, H: 10}, white)

	if got := img.At(5, 5); got != white {
		t.Errorf("the first pixel inside is %v, want white", got)
	}
	if got := img.At(14, 14); got != white {
		t.Errorf("the last pixel inside is %v, want white", got)
	}
	if got := img.At(4, 5); got != black {
		t.Errorf("a pixel left of the rectangle is %v, want black", got)
	}
	if got := img.At(15, 5); got != black {
		t.Errorf("a pixel right of the rectangle is %v, want black", got)
	}
}

// FillRect writes one row and copies it into the rest, so a rectangle narrower than the image is
// the case that catches a wrong offset: the copies would land shifted, or run past the right edge
// onto the next row.
func TestFillRectCopiesRowsToTheSamePlace(t *testing.T) {
	img := NewImage(20, 20, black)
	FillRect(img, Rect{X: 6, Y: 2, W: 7, H: 9}, white)

	for y := range 20 {
		for x := range 20 {
			want := black
			if x >= 6 && x < 13 && y >= 2 && y < 11 {
				want = white
			}
			if got := img.At(x, y); got != want {
				t.Fatalf("pixel %d,%d is %v, want %v", x, y, got, want)
			}
		}
	}
}

// A rectangle of one row copies nothing, which is the other side of the same code.
func TestFillRectOfOneRow(t *testing.T) {
	img := NewImage(8, 4, black)
	FillRect(img, Rect{X: 1, Y: 2, W: 5, H: 1}, white)

	for x := range 8 {
		want := black
		if x >= 1 && x < 6 {
			want = white
		}
		if got := img.At(x, 2); got != want {
			t.Errorf("pixel %d,2 is %v, want %v", x, got, want)
		}
		if got := img.At(x, 1); got != black {
			t.Errorf("pixel %d,1 is %v, want the row above untouched", x, got)
		}
	}
}

// An empty rectangle paints nothing rather than one stray row.
func TestFillRectOfNothing(t *testing.T) {
	for _, r := range []Rect{{}, {X: 2, Y: 2}, {X: 2, Y: 2, W: 4}, {X: 2, Y: 2, H: 4}, {X: 30, Y: 30, W: 4, H: 4}} {
		img := NewImage(8, 4, black)
		FillRect(img, r, white)

		for y := range 4 {
			for x := range 8 {
				if got := img.At(x, y); got != black {
					t.Fatalf("%+v painted %d,%d", r, x, y)
				}
			}
		}
	}
}

// Drawing off the edge must clip rather than wrap onto the next row, which is what an unclipped
// write into a flat buffer does.
func TestFillRectClips(t *testing.T) {
	img := NewImage(10, 10, black)
	FillRect(img, Rect{X: -5, Y: -5, W: 20, H: 20}, white)

	for y := range 10 {
		for x := range 10 {
			if got := img.At(x, y); got != white {
				t.Fatalf("pixel %d,%d is %v, want the whole image filled", x, y, got)
			}
		}
	}
}

func TestFill(t *testing.T) {
	img := NewImage(8, 4, black)
	Fill(img, red)

	for y := range 4 {
		for x := range 8 {
			if got := img.At(x, y); got != red {
				t.Fatalf("pixel %d,%d is %v, want red", x, y, got)
			}
		}
	}
}

// A rounded rectangle keeps its edges and loses its corners, which is the whole point of it.
func TestFillRounded(t *testing.T) {
	img := NewImage(40, 40, black)
	r := Rect{X: 5, Y: 5, W: 30, H: 30}
	FillRounded(img, r, 8, white)

	if got := img.At(20, 5); got != white {
		t.Errorf("the middle of the top edge is %v, want white", got)
	}
	if got := img.At(5, 20); got != white {
		t.Errorf("the middle of the left edge is %v, want white", got)
	}
	if got := img.At(20, 20); got != white {
		t.Errorf("the center is %v, want white", got)
	}
	if got := img.At(5, 5); got != black {
		t.Errorf("the top-left corner is %v, want it rounded away", got)
	}
	if got := img.At(34, 34); got != black {
		t.Errorf("the bottom-right corner is %v, want it rounded away", got)
	}
}

// A radius larger than the shape would otherwise fold the corners through each other.
func TestFillRoundedClampsTheRadius(t *testing.T) {
	img := NewImage(20, 20, black)
	FillRounded(img, Rect{X: 2, Y: 2, W: 10, H: 10}, 100, white)

	if got := img.At(7, 7); got != white {
		t.Errorf("the center is %v, want white", got)
	}
}

func TestFillRoundedWithNoRadiusIsARectangle(t *testing.T) {
	img := NewImage(20, 20, black)
	FillRounded(img, Rect{X: 5, Y: 5, W: 10, H: 10}, 0, white)

	if got := img.At(5, 5); got != white {
		t.Errorf("the corner is %v, want a square corner", got)
	}
}

func TestBorder(t *testing.T) {
	img := NewImage(20, 20, black)
	r := Rect{X: 2, Y: 2, W: 16, H: 16}
	Border(img, r, 2, white)

	if got := img.At(2, 2); got != white {
		t.Errorf("the corner of the border is %v, want white", got)
	}
	if got := img.At(9, 3); got != white {
		t.Errorf("the top edge is %v, want white", got)
	}
	if got := img.At(9, 9); got != black {
		t.Errorf("the middle is %v, want it left alone", got)
	}
	if got := img.At(17, 9); got != white {
		t.Errorf("the right edge is %v, want white", got)
	}
}

func TestDraw(t *testing.T) {
	src := NewImage(4, 4, red)
	dst := NewImage(20, 20, black)

	Draw(dst, src, 8, 8)

	if got := dst.At(8, 8); got != red {
		t.Errorf("the corner of the copy is %v, want red", got)
	}
	if got := dst.At(11, 11); got != red {
		t.Errorf("the far corner of the copy is %v, want red", got)
	}
	if got := dst.At(12, 12); got != black {
		t.Errorf("past the copy is %v, want black", got)
	}
}

func TestDrawClips(t *testing.T) {
	src := NewImage(10, 10, red)
	dst := NewImage(8, 8, black)

	Draw(dst, src, 5, 5)

	if got := dst.At(7, 7); got != red {
		t.Errorf("the last pixel is %v, want red", got)
	}
}

func TestImageIgnoresWritesOutside(t *testing.T) {
	img := NewImage(4, 4, black)

	img.Set(-1, 0, white)
	img.Set(0, -1, white)
	img.Set(4, 0, white)
	img.Set(0, 4, white)

	for y := range 4 {
		for x := range 4 {
			if got := img.At(x, y); got != black {
				t.Fatalf("pixel %d,%d is %v, want the image untouched", x, y, got)
			}
		}
	}
}

func TestLoadCachesFaces(t *testing.T) {
	a, err := Load(Regular, 32)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	b, err := Load(Regular, 32)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if a != b {
		t.Error("the same font at the same size was rasterized twice")
	}

	c, err := Load(Bold, 32)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a == c {
		t.Error("two weights came back as one font")
	}
}

func TestMeasureGrowsWithTheText(t *testing.T) {
	f := MustLoad(Regular, 32)

	short, _ := f.Measure("1")
	long, _ := f.Measure("12:34")

	if short <= 0 {
		t.Fatalf("a single character measured %d", short)
	}
	if long <= short {
		t.Errorf("%q measured %d, no wider than %q at %d", "12:34", long, "1", short)
	}
}

func TestMeasureGrowsWithTheSize(t *testing.T) {
	small := MustLoad(Regular, 16)
	large := MustLoad(Regular, 64)

	sw, sh := small.Measure("12:34")
	lw, lh := large.Measure("12:34")

	if lw <= sw || lh <= sh {
		t.Errorf("at 64 the text is %dx%d, no bigger than %dx%d at 16", lw, lh, sw, sh)
	}
}

func TestMeasureOfNothingIsNothing(t *testing.T) {
	if w, _ := MustLoad(Regular, 32).Measure(""); w != 0 {
		t.Errorf("an empty string measured %d wide", w)
	}
}

func TestDrawTextMarksThePage(t *testing.T) {
	f := MustLoad(Bold, 40)
	img := NewImage(200, 80, black)

	DrawText(img, f, 10, 10, white, black, "12:34")

	var lit int
	for y := range 80 {
		for x := range 200 {
			if img.At(x, y) != black {
				lit++
			}
		}
	}
	if lit == 0 {
		t.Fatal("drawing text left the image blank")
	}

	// Loosely bounded: enough to be lettering, not so much that it filled the page.
	if lit > 200*80/2 {
		t.Errorf("%d pixels were painted, which is more than lettering", lit)
	}
}

// Text has to stay inside what it measured, or a layout built on Measure overlaps.
func TestDrawTextStaysWithinItsMeasure(t *testing.T) {
	f := MustLoad(Regular, 30)
	const s = "Kitchen"

	w, h := f.Measure(s)
	img := NewImage(w+40, h+40, black)
	DrawText(img, f, 20, 20, white, black, s)

	for y := range img.h {
		for x := range img.w {
			if img.At(x, y) == black {
				continue
			}
			if x < 20 || x >= 20+w || y < 20 || y >= 20+h {
				t.Fatalf("a pixel at %d,%d is outside the measured %dx%d box at 20,20", x, y, w, h)
			}
		}
	}
}

func TestDrawTextInCentres(t *testing.T) {
	f := MustLoad(Regular, 20)
	img := NewImage(200, 100, black)
	r := Rect{W: 200, H: 100}

	DrawTextIn(img, f, r, white, black, "ok")

	// The painted pixels should straddle the middle rather than sitting against an edge.
	minX, maxX := 1<<30, -1
	for y := range 100 {
		for x := range 200 {
			if img.At(x, y) != black {
				minX = min(minX, x)
				maxX = max(maxX, x)
			}
		}
	}
	if maxX < 0 {
		t.Fatal("nothing was drawn")
	}

	center := (minX + maxX) / 2
	if center < 90 || center > 110 {
		t.Errorf("the text is centered at %d, want near 100", center)
	}
}

func TestDrawTextRightIsFlushRight(t *testing.T) {
	f := MustLoad(Regular, 20)
	img := NewImage(200, 60, black)

	DrawTextRight(img, f, Rect{W: 190, H: 60}, white, black, "42")

	maxX := -1
	for y := range 60 {
		for x := range 200 {
			if img.At(x, y) != black {
				maxX = max(maxX, x)
			}
		}
	}
	if maxX < 0 {
		t.Fatal("nothing was drawn")
	}
	if maxX < 180 || maxX > 190 {
		t.Errorf("the text ends at %d, want it against the right edge at 190", maxX)
	}
}

// Drawing off the edge of the panel must clip rather than wrap.
func TestDrawTextClips(t *testing.T) {
	f := MustLoad(Regular, 40)
	img := NewImage(40, 40, black)

	DrawText(img, f, -20, -20, white, black, "88:88")
	DrawText(img, f, 35, 35, white, black, "88:88")
}
