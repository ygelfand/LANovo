package ui

import (
	"image"
	"testing"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// A stroked circle: a ring, not a disc. This is the shape the icon tool refuses, because IconVG
// carries one channel of coverage and has nowhere to put a stroke — arcticons is drawn this way
// throughout, so it is the whole reason for the second path.
const ring = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
  <circle cx="12" cy="12" r="9" fill="none" stroke="black" stroke-width="2"/>
</svg>`

// A filled square, for the cases where coverage is all there is either way.
const block = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
  <rect x="4" y="4" width="16" height="16" fill="black"/>
</svg>`

func TestTheTwoFormatsAreToldApartByTheirFirstBytes(t *testing.T) {
	// The header an IconVG file opens with. The set lives in ui/mark, which imports this package,
	// so the bytes are here rather than an icon from it.
	if !IsVG(Icon{0x89, 'I', 'V', 'G', 0x02, 0x0a}) {
		t.Error("an iconvg icon was not recognised as one")
	}
	if IsVG(Icon(ring)) {
		t.Error("an svg icon was taken for iconvg")
	}
	if IsVG(nil) {
		t.Error("nothing was taken for iconvg")
	}
}

// The ring has to be hollow. A renderer that filled it would still produce a plausible icon, which
// is why this checks the middle rather than eyeballing the edges.
func TestAStrokedIconKeepsItsHole(t *testing.T) {
	const side = 48

	m := mask(Icon(ring), side, side)

	middle := m.Pix[side/2*m.Stride+side/2]
	if middle != 0 {
		t.Errorf("the centre of a stroked ring has %d coverage, want none", middle)
	}

	// The stroke itself, a little in from the edge. r=9 of 24 scaled to 48 puts it 18 out from the
	// centre, so the row through the middle crosses it at x = 24-18 = 6.
	var found bool
	for x := 2; x < 12; x++ {
		if m.Pix[side/2*m.Stride+x] > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("the ring's stroke drew nothing")
	}
}

// A filled shape covers its middle, which is the case that already worked and must keep working.
func TestAFilledIconCoversItsMiddle(t *testing.T) {
	const side = 48

	m := mask(Icon(block), side, side)
	if got := m.Pix[side/2*m.Stride+side/2]; got != 0xff {
		t.Errorf("the centre of a filled square has %d coverage, want full", got)
	}
}

// An icon that is neither format draws nothing rather than taking the screen down, the same way a
// bad IconVG one does.
func TestRubbishDrawsNothing(t *testing.T) {
	const side = 16

	m := mask(Icon("<svg>this is not finished"), side, side)
	if m == nil {
		t.Fatal("a broken icon gave no mask at all")
	}
	for i, v := range m.Pix {
		if v != 0 {
			t.Fatalf("byte %d of a broken icon's mask is %d", i, v)
		}
	}
}

// The mask is cached on the icon and the size together, so a second size is rasterized rather than
// stretched from the first.
func TestEachSizeIsItsOwn(t *testing.T) {
	small := mask(Icon(block), 16, 16)
	large := mask(Icon(block), 64, 64)

	if len(small.Pix) == len(large.Pix) {
		t.Errorf("both sizes came back as %d bytes", len(small.Pix))
	}
	if again := mask(Icon(block), 64, 64); again != large {
		t.Error("the same icon at the same size was rasterized twice")
	}
}

// Two colors in one icon, which is the case a coverage mask cannot carry: a silhouette of a logo
// is not a logo.
const twoColor = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
  <rect x="0" y="0" width="12" height="24" fill="#ff0000"/>
  <rect x="12" y="0" width="12" height="24" fill="#0000ff"/>
</svg>`

func TestALogoKeepsItsColors(t *testing.T) {
	const side = 24

	s := NewImage(side, side, theme.Color{})
	DrawColored(s, Icon(twoColor), Rect{W: side, H: side}, theme.Color{})

	left := s.At(side/4, side/2)
	right := s.At(side*3/4, side/2)

	if left.R < 0xc0 || left.G > 0x40 || left.B > 0x40 {
		t.Errorf("the left half came out %+v, want red", left)
	}
	if right.B < 0xc0 || right.R > 0x40 || right.G > 0x40 {
		t.Errorf("the right half came out %+v, want blue", right)
	}
}

// The coverage of the same icon is one channel, so both halves read as solid and the color is
// gone. That is the whole difference between the two calls.
func TestTheSameIconAsCoverageLosesThem(t *testing.T) {
	const side = 24

	m := mask(Icon(twoColor), side, side)
	if got := m.Pix[side/2*m.Stride+side/4]; got != 0xff {
		t.Errorf("the left half has %d coverage, want full", got)
	}
	if got := m.Pix[side/2*m.Stride+side*3/4]; got != 0xff {
		t.Errorf("the right half has %d coverage, want full", got)
	}
}

// The two caches are counted against one budget, and dropping a size has to drop both or the
// accounting drifts until nothing fits.
func TestBothCachesAreEvictedTogether(t *testing.T) {
	iconMu.Lock()
	iconCache, colorCache, iconBytes = map[iconKey]*image.Alpha{}, map[iconKey]*image.RGBA{}, 0
	iconMu.Unlock()

	mask(Icon(block), 32, 32)
	inColor(Icon(twoColor), 32, 32)

	iconMu.Lock()
	before := iconBytes
	iconMu.Unlock()
	if before == 0 {
		t.Fatal("nothing was counted")
	}

	// A different size, which is what makes the old one dead.
	iconMu.Lock()
	evict(iconKey{w: 64, h: 64})
	after, alphas, colors := iconBytes, len(iconCache), len(colorCache)
	iconMu.Unlock()

	if alphas != 0 || colors != 0 {
		t.Errorf("%d coverage and %d color entries survived a size change", alphas, colors)
	}
	if after != 0 {
		t.Errorf("%d bytes still counted after both caches were emptied", after)
	}
}
