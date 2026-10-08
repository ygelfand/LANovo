package takeover

import "testing"

func TestSettingAPropertyRewritesItsLine(t *testing.T) {
	in := "#\nro.secure=1\nro.debuggable=1\n"

	out, changed := setProp([]byte(in), "ro.secure", "0")
	if !changed {
		t.Fatal("rewriting ro.secure=1 reports nothing changed")
	}
	if got, want := string(out), "#\nro.secure=0\nro.debuggable=1\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSettingAPropertyAlreadySetChangesNothing(t *testing.T) {
	in := "ro.secure=0\n"

	out, changed := setProp([]byte(in), "ro.secure", "0")
	if changed {
		t.Error("a property already at the wanted value reports a change")
	}
	if string(out) != in {
		t.Errorf("the file came back as %q, want it untouched", out)
	}
}

func TestSettingAPropertyThatIsNotThereAppendsIt(t *testing.T) {
	out, changed := setProp([]byte("ro.debuggable=1\n"), "ro.secure", "0")
	if !changed {
		t.Fatal("appending a missing property reports nothing changed")
	}
	if got, want := string(out), "ro.debuggable=1\nro.secure=0\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAppendingToAFileWithNoTrailingNewline(t *testing.T) {
	out, changed := setProp([]byte("ro.debuggable=1"), "ro.secure", "0")
	if !changed {
		t.Fatal("appending reports nothing changed")
	}
	if got, want := string(out), "ro.debuggable=1\nro.secure=0\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestALongerKeyWithTheSamePrefixIsNotMistaken(t *testing.T) {
	in := "ro.secureboot=1\nro.secure=1\n"

	out, _ := setProp([]byte(in), "ro.secure", "0")
	if got, want := string(out), "ro.secureboot=1\nro.secure=0\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
