package config

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

func TestDefaultsAreUsable(t *testing.T) {
	c := Defaults()

	if c.Screen.Backlight != DefaultBacklight {
		t.Errorf("backlight defaults to %d, want %d", c.Screen.Backlight, DefaultBacklight)
	}
	if c.Screen.Mode != DefaultScreenMode {
		t.Errorf("screen mode defaults to %q, want %q", c.Screen.Mode, DefaultScreenMode)
	}
	if c.Diag.Interval != DefaultInterval {
		t.Errorf("metrics interval defaults to %d, want %d", c.Diag.Interval, DefaultInterval)
	}
}

// A device that has never been configured reads its defaults rather than failing to start.
func TestLoadWithNoFile(t *testing.T) {
	st, err := Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got := st.Get(); got.Screen.Backlight != DefaultBacklight {
		t.Errorf("backlight = %d, want the default %d", got.Screen.Backlight, DefaultBacklight)
	}
}

// The whole point of the store: a setting survives the process that set it.
func TestSettingPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	st, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.Set().Screen().Backlight(42); err != nil {
		t.Fatalf("Backlight: %v", err)
	}
	if err := st.Set().Diag().Interval(120); err != nil {
		t.Fatalf("Interval: %v", err)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}

	got := again.Get()
	if got.Screen.Backlight != 42 {
		t.Errorf("backlight came back %d, want 42", got.Screen.Backlight)
	}
	if got.Diag.Interval != 120 {
		t.Errorf("interval came back %d, want 120", got.Diag.Interval)
	}
}

// A setting nobody has touched keeps its default when another is written, rather than coming back
// as a zero.
func TestWritingOneSettingLeavesTheRest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	st, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := st.Set().Diag().Interval(120); err != nil {
		t.Fatalf("Interval: %v", err)
	}

	again, _ := Load(path)
	if got := again.Get(); got.Screen.Backlight != DefaultBacklight {
		t.Errorf("backlight came back %d, want the default %d", got.Screen.Backlight, DefaultBacklight)
	}
}

// Device is what the process was told, not what anyone chose, so it is not written to the file.
func TestDeviceIsNotPersisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	st, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	st.started(Device{Name: "Kitchen", Addr: ":6053"})

	if err := st.Set().Screen().Backlight(10); err != nil {
		t.Fatalf("Backlight: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if got := string(b); contains(got, "Kitchen") {
		t.Errorf("the saved config names the device:\n%s", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestLabelsRoundTrip(t *testing.T) {
	modes := ScreenModes()
	labels := Labels(modes)

	if len(labels) != len(modes) {
		t.Fatalf("%d labels for %d modes", len(labels), len(modes))
	}

	for i, mode := range modes {
		got, ok := ByLabel(modes, labels[i])
		if !ok {
			t.Errorf("label %q resolves to nothing", labels[i])
			continue
		}
		if got != mode {
			t.Errorf("label %q resolved to %q, want %q", labels[i], got, mode)
		}
	}
}

// Home Assistant can send anything; an unknown label must not resolve to the first option.
func TestUnknownLabelIsRejected(t *testing.T) {
	if _, ok := ByLabel(ScreenModes(), "Nonesuch"); ok {
		t.Error("an unknown label resolved to a mode")
	}
}

// Settings are changed from several places at once — Home Assistant, the buttons, the drawer — and
// all of them write the whole document through one temporary file. Overlapping writes must not
// leave something that does not parse, because a store that cannot read its file will not write to
// it either, and the device silently stops saving anything.
func TestConcurrentUpdatesLeaveAReadableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	st, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range 25 {
				level := (i*25 + n) % 101
				if err := (ScreenWriter{st}).Backlight(level); err != nil {
					t.Errorf("Backlight: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	again, err := Load(path)
	if err != nil {
		t.Fatalf("the file left behind does not parse: %v", err)
	}
	if got := again.Get().Screen.Backlight; got < 0 || got > 100 {
		t.Errorf("backlight came back as %d, want a level something actually set", got)
	}
}

// The last change to be made is the one on disk, not whichever write finished last.
func TestUpdateWritesTheNewestState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	st, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, level := range []int{10, 20, 30, 40, 50} {
		if err := (ScreenWriter{st}).Backlight(level); err != nil {
			t.Fatalf("Backlight: %v", err)
		}
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := again.Get().Screen.Backlight; got != 50 {
		t.Errorf("backlight = %d, want the last one set, 50", got)
	}
}

// A device nobody has added shows the code to scan, so the default has to be false: defaulting the
// other way hides the one screen that explains how to add it.
func TestAdoptionDefaultsToFalse(t *testing.T) {
	if Defaults().API.Adopted {
		t.Error("a device out of the box thinks it has been adopted")
	}
}

// Adoption is what the onboarding screen waits on, so it has to outlive the process that recorded
// it — otherwise every restart puts the code back on a device that has been added.
func TestAdoptionPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	st, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := (APIWriter{st}).Adopted(true); err != nil {
		t.Fatalf("Adopted: %v", err)
	}

	again, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !again.Get().API.Adopted {
		t.Error("adoption did not survive a restart")
	}
}

// Every label has to be distinct, or a select shows two rows that do the same thing and one of
// them cannot be chosen.
func TestLabelsAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, label := range Labels(ScreenModes()) {
		if seen[label] {
			t.Errorf("two screen modes are labeled %q", label)
		}
		seen[label] = true
	}
}

func TestSponsorBlockSkipsSponsorsUntilEmptied(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	st, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Get().Cast.YouTube.Skip; !slices.Equal(got, []string{"sponsor"}) {
		t.Errorf("default %v", got)
	}

	if err := st.Set().Cast().Skip(nil); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Get().Cast.YouTube.Skip; len(got) != 0 {
		t.Errorf("emptied came back %v", got)
	}

	if err := again.Set().Cast().Receiver(true); err != nil {
		t.Fatal(err)
	}
	if got := again.Get().Cast.YouTube.Skip; len(got) != 0 {
		t.Errorf("another write brought back %v", got)
	}
}
