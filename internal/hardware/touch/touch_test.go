package touch

import (
	"io/fs"
	"slices"
	"testing"

	"github.com/ygelfand/LANovo/internal/board"
)

func TestAmberTouchSelection(t *testing.T) {
	for _, tc := range []struct {
		present []string
		want    string
	}{
		{[]string{"himax-touchscreen"}, "himax-touchscreen"},
		{[]string{"Elan Touchscreen"}, "Elan Touchscreen"},
		{[]string{"Elan Touchscreen", "himax-touchscreen"}, "himax-touchscreen"},
	} {
		got := device(board.Amber.Touch, func(name string) (string, error) {
			if slices.Contains(tc.present, name) {
				return "/dev/input/event1", nil
			}
			return "", fs.ErrNotExist
		})
		if got != tc.want {
			t.Errorf("present %v: selected %q, want %q", tc.present, got, tc.want)
		}
	}
}
