package ui

import (
	"image"
	"testing"

	"golang.org/x/exp/shiny/materialdesign/icons"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// empty starts the cache from nothing, since these are about what it holds rather than what it
// draws and every other test in the package has been filling it.
func empty() {
	iconMu.Lock()
	defer iconMu.Unlock()

	iconCache = map[iconKey]*image.Alpha{}
	iconBytes = 0
}

func held() (entries, bytes int) {
	iconMu.Lock()
	defer iconMu.Unlock()

	return len(iconCache), iconBytes
}

// The same icon at the same size is rasterized once. This is the whole reason the cache exists and
// nothing else in the package asserts it.
func TestTheSameMaskIsNotRasterizedTwice(t *testing.T) {
	empty()

	mask(icons.ActionInfoOutline, 64, 64)
	was, _ := held()

	mask(icons.ActionInfoOutline, 64, 64)
	now, _ := held()

	if was != 1 || now != 1 {
		t.Errorf("the cache holds %d entries after one mask and %d after asking twice", was, now)
	}
}

// A numeral is not square, so the same artwork at the same height and a different width is a
// different mask. A cache keyed on one side would hand back the wrong shape.
func TestBothSidesArePartOfTheKey(t *testing.T) {
	empty()

	mask(icons.ActionInfoOutline, 64, 64)
	mask(icons.ActionInfoOutline, 32, 64)

	if got, _ := held(); got != 2 {
		t.Errorf("two shapes of the same icon made %d entries", got)
	}
}

// The cache has a ceiling. Without one a clock drawing large numerals accumulates every digit it
// has ever shown, at every size it has ever been set to, and never gives any of it back.
func TestTheCacheStaysUnderItsBudget(t *testing.T) {
	empty()

	// Sizes chosen to overrun several times over: each of these is about a megabyte of coverage.
	for _, side := range []int{1000, 1010, 1020, 1030, 1040, 1050} {
		mask(icons.ActionInfoOutline, side, side)

		if _, bytes := held(); bytes > iconBudget {
			t.Fatalf("holding %d bytes, over the %d budget", bytes, iconBudget)
		}
	}
}

// What it gives back is every size that is not the one on screen. Changing size is what makes the
// old masks dead, all at once.
func TestChangingSizeDropsTheOldMasks(t *testing.T) {
	empty()

	// Four icons at one size, enough that dropping them is measurable.
	set := []Icon{
		icons.ActionInfoOutline,
		icons.ActionSettings,
		icons.AlertWarning,
		icons.ContentAdd,
	}
	for _, i := range set {
		mask(i, 1000, 1000)
	}

	before, _ := held()
	if before != len(set) {
		t.Fatalf("the cache holds %d of %d masks", before, len(set))
	}

	// One at a different size, big enough to push the total past the budget.
	mask(icons.ActionInfoOutline, 1100, 1100)

	entries, bytes := held()
	if bytes > iconBudget {
		t.Errorf("holding %d bytes, over the %d budget", bytes, iconBudget)
	}
	if entries >= before {
		t.Errorf("the cache holds %d entries, want fewer than the %d at the old size",
			entries, before)
	}

	iconMu.Lock()
	defer iconMu.Unlock()
	for at := range iconCache {
		if at.w == 1000 {
			t.Errorf("a mask at the old size is still held: %v", at)
		}
	}
}

// Evicting must not lose what was just asked for, or every draw rasterizes again and the cache is
// a pure cost.
func TestTheMaskBeingAskedForSurvivesEviction(t *testing.T) {
	empty()

	for _, side := range []int{900, 950, 1000, 1050} {
		m := mask(icons.ActionInfoOutline, side, side)

		iconMu.Lock()
		kept, ok := iconCache[iconKey{icon: string(icons.ActionInfoOutline), w: side, h: side}]
		iconMu.Unlock()

		if !ok || kept != m {
			t.Fatalf("at %d the mask just made is not the one held", side)
		}
	}
}

// The budget is counted, not estimated: what is subtracted on the way out has to be what was added
// on the way in, or it drifts until the cache holds nothing or everything.
func TestTheByteCountMatchesWhatIsHeld(t *testing.T) {
	empty()

	for _, side := range []int{48, 64, 900, 64, 1000} {
		mask(icons.ActionSettings, side, side)
	}

	iconMu.Lock()
	defer iconMu.Unlock()

	var want int
	for _, m := range iconCache {
		want += len(m.Pix)
	}
	if iconBytes != want {
		t.Errorf("the count says %d bytes, the cache holds %d", iconBytes, want)
	}
}

// Drawing still works when the draw is the thing that triggers an eviction. The mask it needs is
// made after the cache has been emptied under it, which is the order that would otherwise hand back
// something that had just been deleted.
func TestDrawingThroughAnEvictionStillPaints(t *testing.T) {
	empty()
	palette := theme.Default()

	// Fill to just under the budget at one size, so the next size over tips it.
	for _, i := range []Icon{icons.ActionInfoOutline, icons.ActionSettings, icons.AlertWarning} {
		mask(i, 1100, 1100)
	}

	img := NewImage(96, 96, palette.Background)
	DrawIcon(img, icons.ActionInfoOutline, Rect{W: 96, H: 96}, palette.Text, palette.Background)

	var lit int
	for y := range 96 {
		for x := range 96 {
			if img.At(x, y) != palette.Background {
				lit++
			}
		}
	}
	if lit == 0 {
		t.Error("nothing was drawn")
	}

	if _, bytes := held(); bytes > iconBudget {
		t.Errorf("holding %d bytes, over the %d budget", bytes, iconBudget)
	}
}

// A mask too big to fit the budget at all is still drawn. It cannot be kept, and the drawing must
// not depend on it having been.
func TestAMaskLargerThanTheBudgetIsStillDrawn(t *testing.T) {
	empty()
	palette := theme.Default()

	// The surface is the middle of the box the icon fills, where its ink actually is.
	const side = 2200

	img := NewImage(160, 160, palette.Background)
	DrawIconFilling(img, icons.ActionInfoOutline,
		Rect{X: -side/2 + 80, Y: -side/2 + 80, W: side, H: side},
		palette.Text, palette.Background)

	for y := range 160 {
		for x := range 160 {
			if img.At(x, y) != palette.Background {
				return
			}
		}
	}
	t.Error("nothing was drawn")
}
