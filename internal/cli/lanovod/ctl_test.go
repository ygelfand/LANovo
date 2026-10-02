package lanovod

import (
	"slices"
	"strings"
	"testing"
)

func TestOneCommandIsOneCommand(t *testing.T) {
	got, err := sequence(strings.NewReader(""), []string{"swipe", "1900", "600", "1400", "600"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"swipe 1900 600 1400 600"}) {
		t.Errorf("got %q", got)
	}
}

// A sequence is written the way somebody types it, with whatever spacing falls out of quoting it
// for a shell.
func TestSemicolonsSeparateCommands(t *testing.T) {
	got, err := sequence(strings.NewReader(""), []string{"tap 100 200 ;  wait 400;shot /tmp/a.png"})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"tap 100 200", "wait 400", "shot /tmp/a.png"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A trailing semicolon is how a list gets typed, and an empty command would be sent as a blank line
// the daemon answers ok to, making the count of answers disagree with the count of commands.
func TestEmptyPartsAreDropped(t *testing.T) {
	got, err := sequence(strings.NewReader(""), []string{"size ; ; "})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"size"}) {
		t.Errorf("got %q", got)
	}
}

// A file of commands keeps its comments and its spacing, because it is read by people as well.
func TestStdinIsACommandToALine(t *testing.T) {
	in := strings.NewReader("# open the drawer\nswipe 1900 600 1400 600\n\n  wait 400  \nshot\n")

	got, err := sequence(in, []string{"-"})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"swipe 1900 600 1400 600", "wait 400", "shot"}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Semicolons in a file are already separated by the lines they are on, and a command that contains
// one — a message body, say — must not be cut in half by it.
func TestStdinDoesNotSplitOnSemicolons(t *testing.T) {
	got, err := sequence(strings.NewReader("message info 5 Hi : there; and again\n"), []string{"-"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"message info 5 Hi : there; and again"}) {
		t.Errorf("got %q", got)
	}
}
