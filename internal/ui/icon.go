package ui

import (
	"image"
	"sync"

	"github.com/ygelfand/LANovo/internal/ui/theme"
)

// Icon is one of the Material Design icons, as IconVG path data.
//
// The set is a package of them, each a few hundred bytes and each its own variable, so a binary
// carries only the ones something names. Nothing here lists them: the names are the library's, and
// a list would be a second one to keep.
type Icon []byte

// rendered is one icon at one size, which is what the drawing actually needs. Icons are redrawn
// every time the screen changes and the rasterizer is not free, so the result is kept.
var (
	iconMu    sync.Mutex
	iconCache = map[iconKey]*image.Alpha{}
	iconBytes int
)

type iconKey struct {
	icon string
	w, h int
}

// iconBudget is how much coverage is worth keeping, in bytes.
//
// An icon beside a settings row is a few kilobytes and a hundred of them would not be worth
// counting. A numeral is the reason this exists: one digit filling a 1200x1920 panel is about half
// a megabyte of alpha, ten of them five, and the clock shows a new one for the first time every few
// minutes for the first hour. Left alone that is five megabytes that are never asked for again the
// moment somebody changes the size.
//
// Four megabytes holds a full set of digits at the largest size the panel can show, plus every icon
// on screen, on a device with 2GB. It is a ceiling rather than a target: nothing reaches it unless
// a face is drawing enormous numerals, which is exactly the case worth bounding.
const iconBudget = 4 << 20

// evict drops everything that is not the size being asked for.
//
// Not a least-recently-used list, because the access pattern does not need one. What fills this
// cache is one face drawing one run of digits at one size, over and over; what makes it stale is
// that size changing, at which point every entry at the old size is dead at once. Keeping the size
// on screen and dropping the rest is the whole policy, and it is one pass with no bookkeeping.
func evict(keep iconKey) {
	for at, m := range iconCache {
		if at.w == keep.w && at.h == keep.h {
			continue
		}
		iconBytes -= len(m.Pix)
		delete(iconCache, at)
	}
	for at, img := range colorCache {
		if at.w == keep.w && at.h == keep.h {
			continue
		}
		if img != nil {
			iconBytes -= len(img.Pix)
		}
		delete(colorCache, at)
	}

	// Still over after dropping every other size, so one size alone does not fit. Start again
	// rather than grow without limit: the entries are about to be rasterized afresh anyway, and a
	// cache that cannot hold a whole frame is not doing anything but holding memory.
	if iconBytes > iconBudget {
		iconCache = map[iconKey]*image.Alpha{}
		colorCache = map[iconKey]*image.RGBA{}
		iconBytes = 0
	}
}

// mask rasterizes an icon to an alpha mask, which is all a single color glyph needs: the shape, to
// be painted in whatever color the caller wants.
//
// Keyed on both sides rather than one. An icon is square and only ever asked for as one, but a
// numeral is not: it is drawn into the box it was designed in, and a cache that remembered only a
// size would hand back the wrong shape for the same digit at a different aspect.
func mask(icon Icon, w, h int) *image.Alpha {
	at := iconKey{icon: string(icon), w: w, h: h}

	iconMu.Lock()
	defer iconMu.Unlock()

	if m, ok := iconCache[at]; ok {
		return m
	}

	rgba, err := raster(icon, w, h)
	if err != nil {
		// A bad icon is a build problem, not something a device recovers from, and a blank mask
		// draws nothing rather than taking the screen down.
		blank := image.NewAlpha(image.Rect(0, 0, w, h))

		if iconBytes+len(blank.Pix) > iconBudget {
			evict(at)
		}
		iconCache[at] = blank
		iconBytes += len(blank.Pix)
		return blank
	}

	m := image.NewAlpha(rgba.Bounds())
	for i, p := 0, 0; i < len(m.Pix); i, p = i+1, p+4 {
		m.Pix[i] = rgba.Pix[p+3]
	}

	if iconBytes+len(m.Pix) > iconBudget {
		evict(at)
	}

	iconCache[at] = m
	iconBytes += len(m.Pix)
	return m
}

// DrawIcon paints an icon in a color, fitted to a square in the middle of r.
//
// on is what it is being drawn over, because a surface is written pixel by pixel with no alpha:
// the edges are mixed against that color here rather than blended by the panel.
func DrawIcon(s Surface, icon Icon, r Rect, fg, on theme.Color) {
	size := min(r.W, r.H)
	if size <= 0 {
		return
	}

	stamp(s, mask(icon, size, size),
		r.X+(r.W-size)/2, r.Y+(r.H-size)/2, size, size, fg, on)
}

// DrawIconFilling paints an icon stretched to the whole of r, rather than fitted to a square in the
// middle of it.
//
// For artwork whose own box is not square and is the point: a numeral is drawn into a design box,
// and squaring it would make the digits narrower than they were drawn and leave the time with gaps
// in it where the design had none.
func DrawIconFilling(s Surface, icon Icon, r Rect, fg, on theme.Color) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	stamp(s, mask(icon, r.W, r.H), r.X, r.Y, r.W, r.H, fg, on)
}

// stamp paints a mask in a color at a position, clipped to the surface.
func stamp(s Surface, m *image.Alpha, x0, y0, mw, mh int, fg, on theme.Color) {
	w, h := s.Size()

	for y := range mh {
		for x := range mw {
			a := m.Pix[y*m.Stride+x]
			if a == 0 {
				continue
			}

			px, py := x0+x, y0+y
			if px < 0 || py < 0 || px >= w || py >= h {
				continue
			}
			s.Set(px, py, on.Mix(fg, a))
		}
	}
}
