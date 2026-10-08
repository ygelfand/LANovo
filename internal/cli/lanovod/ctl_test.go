package lanovod

import (
	"slices"
	"strings"
	"testing"
)

func TestOneCommandIsOneCommand(t *testing.T) {
	got, err := sequence(
		strings.NewReader(""),
		[]string{"input", "swipe", "1900", "600", "1400", "600"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"input swipe 1900 600 1400 600"}) {
		t.Errorf("got %q", got)
	}
}

func TestSemicolonsSeparateCommands(t *testing.T) {
	got, err := sequence(
		strings.NewReader(""),
		[]string{"input tap 100 200 ;  device wait 400;display shot /tmp/a.png"},
	)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"input tap 100 200", "device wait 400", "display shot /tmp/a.png"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEmptyPartsAreDropped(t *testing.T) {
	got, err := sequence(strings.NewReader(""), []string{"display size ; ; "})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"display size"}) {
		t.Errorf("got %q", got)
	}
}

func TestStdinIsACommandToALine(t *testing.T) {
	in := strings.NewReader(
		"# open the drawer\ninput swipe 1900 600 1400 600\n\n  device wait 400  \ndisplay shot\n",
	)

	got, err := sequence(in, []string{"-"})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"input swipe 1900 600 1400 600", "device wait 400", "display shot"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStdinDoesNotSplitOnSemicolons(t *testing.T) {
	got, err := sequence(
		strings.NewReader("display message info 5 Hi : there; and again\n"),
		[]string{"-"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"display message info 5 Hi : there; and again"}) {
		t.Errorf("got %q", got)
	}
}
