package call

import (
	"testing"

	"github.com/ygelfand/LANovo/internal/feature/livecam"
	"github.com/ygelfand/LANovo/internal/hardware/display"
)

func inside(r, box display.Rect) bool {
	return r.X >= box.X && r.Y >= box.Y && r.X+r.W <= box.X+box.W && r.Y+r.H <= box.Y+box.H
}

func overlaps(a, b display.Rect) bool {
	return a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H
}

func TestArrange(t *testing.T) {
	cases := []struct {
		name        string
		vw, vh      int
		remote, own livecam.Size
		side        bool
	}{
		{"portrait screen, landscape caller", 1200, 1920, livecam.Size{Width: 768, Height: 432}, livecam.Size{Width: 480, Height: 640}, true},
		{"landscape screen, portrait caller", 1024, 600, livecam.Size{Width: 480, Height: 640}, livecam.Size{Width: 768, Height: 432}, true},
		{"landscape screen, landscape caller", 1024, 600, livecam.Size{Width: 768, Height: 432}, livecam.Size{Width: 768, Height: 432}, false},
		{"portrait screen, portrait caller", 1200, 1920, livecam.Size{Width: 480, Height: 640}, livecam.Size{Width: 480, Height: 640}, false},
		{"size not known yet", 1024, 600, livecam.Size{}, livecam.Size{Width: 768, Height: 432}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			full := display.Rect{W: c.vw, H: c.vh}
			l := arrange(c.vw, c.vh, c.remote, c.own)
			if l.Side != c.side {
				t.Fatalf("side %v, want %v (%+v)", l.Side, c.side, l)
			}
			for name, r := range map[string]display.Rect{"remote": l.Remote, "picture": l.Picture, "self": l.Self, "panel": l.Panel} {
				if !inside(r, full) || r.W <= 0 || r.H <= 0 {
					t.Fatalf("%s %+v is not on the %dx%d screen", name, r, c.vw, c.vh)
				}
			}
			if !inside(l.Picture, l.Remote) {
				t.Fatalf("picture %+v outside remote %+v", l.Picture, l.Remote)
			}
			if c.side {
				if overlaps(l.Remote, l.Panel) || overlaps(l.Self, l.Remote) || !inside(l.Self, l.Panel) {
					t.Fatalf("side layout overlaps: %+v", l)
				}
				if got, want := float64(l.Picture.W)/float64(l.Picture.H), float64(c.remote.Width)/float64(c.remote.Height); got < want*0.98 || got > want*1.02 {
					t.Fatalf("picture shape %.3f, caller sends %.3f", got, want)
				}
			}
		})
	}
}
