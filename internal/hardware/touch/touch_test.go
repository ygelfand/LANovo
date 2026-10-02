package touch

import (
	"runtime"
	"testing"

	"github.com/ygelfand/LANovo/internal/board"
)

// The device's own listing, so a change in the kernel's format is caught rather than assumed.
const procDevices = `I: Bus=0000 Vendor=0000 Product=0000 Version=0000
N: Name="qpnp_pon"
P: Phys=qpnp_pon/input0
S: Sysfs=/devices/virtual/input/input0
U: Uniq=
H: Handlers=event0 cpufreq
B: PROP=0
B: EV=3

I: Bus=0018 Vendor=dead Product=beef Version=28bb
N: Name="goodix-ts"
P: Phys=input/ts
S: Sysfs=/devices/virtual/input/input1
U: Uniq=
H: Handlers=mdss_fb kgsl event1 cpufreq
B: PROP=2
B: EV=b

I: Bus=0000 Vendor=0000 Product=0000 Version=0000
N: Name="msm8953-openq624-snd-card Headset Jack"
P: Phys=ALSA
S: Sysfs=/devices/platform/soc/c051000.sound/sound/card0/input2
U: Uniq=
H: Handlers=event2
B: PROP=0
B: EV=21
`

func TestHandlerFor(t *testing.T) {
	tests := []struct {
		name   string
		device string
		want   string
		wantOK bool
	}{
		{"the touchscreen, past the handlers that are not events", "goodix-ts", "event1", true},
		{"the power key", "qpnp_pon", "event0", true},
		{"a name with spaces", "msm8953-openq624-snd-card Headset Jack", "event2", true},
		{"a device that is not there", "nonesuch", "", false},
		{"a prefix of a real name does not match", "goodix", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := handlerFor(procDevices, tt.device)
			if ok != tt.wantOK {
				t.Fatalf("handlerFor(%q) ok = %v, want %v", tt.device, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("handlerFor(%q) = %q, want %q", tt.device, got, tt.want)
			}
		})
	}
}

// The panel is portrait and mounted landscape, so a touch has to land where Panel.Set drew.
func TestRotate(t *testing.T) {
	tests := []struct {
		name         string
		px, py       int
		wantX, wantY int
	}{
		{"panel origin is the bottom left of the picture", 0, 0, board.Current().PanelHeight - 1, 0},
		{"the far corner of the panel is the top right", board.Current().PanelWidth - 1, board.Current().PanelHeight - 1, 0, board.Current().PanelWidth - 1},
		{"the middle stays in the middle", 600, 960, 959, 600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y := rotate(tt.px, tt.py)
			if x != tt.wantX || y != tt.wantY {
				t.Errorf("rotate(%d, %d) = %d,%d, want %d,%d", tt.px, tt.py, x, y, tt.wantX, tt.wantY)
			}
		})
	}
}

// rotate must land inside what the display calls its screen, or a touch is reported off the edge.
func TestRotateStaysOnScreen(t *testing.T) {
	corners := [][2]int{
		{0, 0},
		{board.Current().PanelWidth - 1, 0},
		{0, board.Current().PanelHeight - 1},
		{board.Current().PanelWidth - 1, board.Current().PanelHeight - 1},
	}

	for _, c := range corners {
		x, y := rotate(c[0], c[1])
		if x < 0 || x >= board.Current().PanelHeight {
			t.Errorf("rotate(%d, %d) x = %d, outside 0..%d", c[0], c[1], x, board.Current().PanelHeight-1)
		}
		if y < 0 || y >= board.Current().PanelWidth {
			t.Errorf("rotate(%d, %d) y = %d, outside 0..%d", c[0], c[1], y, board.Current().PanelWidth-1)
		}
	}
}

func ev(t uint16, code uint16, value int32) rawEvent {
	return rawEvent{Type: t, Code: code, Value: value}
}

func syn() rawEvent { return rawEvent{Type: evSyn, Code: synReport} }

// feed runs a stream through a decoder and collects everything it reported.
func feed(d *decoder, events ...rawEvent) []Contact {
	var got []Contact
	for _, e := range events {
		got = append(got, d.event(e)...)
	}
	return got
}

func TestNothingIsReportedUntilSyn(t *testing.T) {
	var d decoder

	got := feed(&d,
		ev(evAbs, absMTTrackingID, 7),
		ev(evAbs, absMTPositionX, 600),
		ev(evAbs, absMTPositionY, 960),
	)
	if len(got) != 0 {
		t.Fatalf("reported %v before SYN_REPORT", got)
	}

	got = feed(&d, syn())
	if len(got) != 1 || got[0].Phase != Down {
		t.Fatalf("got %v, want one down", got)
	}
}

func TestTapIsDownThenUp(t *testing.T) {
	var d decoder

	down := feed(&d,
		ev(evAbs, absMTTrackingID, 7),
		ev(evAbs, absMTPositionX, 600),
		ev(evAbs, absMTPositionY, 960),
		syn(),
	)
	if len(down) != 1 {
		t.Fatalf("got %v, want one contact", down)
	}
	if down[0].Phase != Down || down[0].ID != 7 || down[0].Slot != 0 {
		t.Errorf("got %v, want a down in slot 0 with id 7", down[0])
	}
	if down[0].X != 959 || down[0].Y != 600 {
		t.Errorf("down at %d,%d, want 959,600", down[0].X, down[0].Y)
	}

	up := feed(&d, ev(evAbs, absMTTrackingID, released), syn())
	if len(up) != 1 || up[0].Phase != Up {
		t.Fatalf("got %v, want one up", up)
	}

	// The id it went down with. A lift reported as -1 matches no journey, so the recognizer drops
	// every gesture and nothing on the screen ever responds to a finger.
	if up[0].ID != 7 {
		t.Errorf("up carries id %d, want the 7 it went down with", up[0].ID)
	}

	// The position a finger was lifted from is the last one reported, which the kernel does not
	// repeat.
	if up[0].X != 959 || up[0].Y != 600 {
		t.Errorf("up at %d,%d, want 959,600", up[0].X, up[0].Y)
	}
}

func TestDragReportsMoves(t *testing.T) {
	var d decoder

	feed(&d, ev(evAbs, absMTTrackingID, 3), ev(evAbs, absMTPositionX, 100), ev(evAbs, absMTPositionY, 200), syn())

	// Only the axis that changed, which is all the kernel sends.
	got := feed(&d, ev(evAbs, absMTPositionY, 300), syn())
	if len(got) != 1 || got[0].Phase != Move {
		t.Fatalf("got %v, want one move", got)
	}

	wantX, wantY := rotate(100, 300)
	if got[0].X != wantX || got[0].Y != wantY {
		t.Errorf("move at %d,%d, want %d,%d", got[0].X, got[0].Y, wantX, wantY)
	}
}

func TestSlotsAreIndependent(t *testing.T) {
	var d decoder

	got := feed(&d,
		ev(evAbs, absMTSlot, 0),
		ev(evAbs, absMTTrackingID, 1),
		ev(evAbs, absMTPositionX, 100),
		ev(evAbs, absMTPositionY, 100),
		ev(evAbs, absMTSlot, 1),
		ev(evAbs, absMTTrackingID, 2),
		ev(evAbs, absMTPositionX, 900),
		ev(evAbs, absMTPositionY, 1800),
		syn(),
	)
	if len(got) != 2 {
		t.Fatalf("got %v, want two contacts", got)
	}

	bySlot := map[int]Contact{}
	for _, c := range got {
		bySlot[c.Slot] = c
	}
	if bySlot[0].ID != 1 || bySlot[1].ID != 2 {
		t.Errorf("slot 0 has id %d and slot 1 has id %d, want 1 and 2", bySlot[0].ID, bySlot[1].ID)
	}

	// Lifting one leaves the other alone.
	got = feed(&d, ev(evAbs, absMTSlot, 0), ev(evAbs, absMTTrackingID, released), syn())
	if len(got) != 1 || got[0].Slot != 0 || got[0].Phase != Up {
		t.Fatalf("got %v, want slot 0 up", got)
	}
}

// A slot the kernel re-uses for a new finger is a new contact, not a jump of the old one.
func TestReusedSlotIsANewDown(t *testing.T) {
	var d decoder

	feed(&d, ev(evAbs, absMTTrackingID, 1), ev(evAbs, absMTPositionX, 10), ev(evAbs, absMTPositionY, 10), syn())
	feed(&d, ev(evAbs, absMTTrackingID, released), syn())

	got := feed(&d, ev(evAbs, absMTTrackingID, 2), ev(evAbs, absMTPositionX, 20), ev(evAbs, absMTPositionY, 20), syn())
	if len(got) != 1 || got[0].Phase != Down || got[0].ID != 2 {
		t.Fatalf("got %v, want a down with id 2", got)
	}
}

// An event size that does not match the kernel's struct reads the stream out of step, which looks
// like a device that works everywhere except the one it ships on.
func TestEventSizeMatchesTheKernelStruct(t *testing.T) {
	want := map[string]int{"arm": 16, "386": 16, "amd64": 24, "arm64": 24}[runtime.GOARCH]
	if want == 0 {
		t.Skipf("no expected size for %s", runtime.GOARCH)
	}
	if eventSize != want {
		t.Errorf("eventSize = %d on %s, want %d", eventSize, runtime.GOARCH, want)
	}
}
