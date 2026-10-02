package control

import "testing"

// One reserved word rather than quoting rules, in a protocol whose point is being typed by hand.
// The cases that matter are the ones somebody will actually type.
func TestTheTitleComesOffAtTheColon(t *testing.T) {
	tests := []struct {
		name        string
		in          []string
		title, body string
	}{
		{
			"both halves",
			[]string{"Back", "door", ":", "It", "has", "been", "open"},
			"Back door", "It has been open",
		},
		{
			"no separator is all body",
			[]string{"The", "kettle", "has", "boiled"},
			"", "The kettle has boiled",
		},
		{
			"a colon inside a word is prose, not a separator",
			[]string{"Back", "door:", "open"},
			"", "Back door: open",
		},
		{
			"a second colon belongs to the body",
			[]string{"Sensors", ":", "hallway", ":", "quiet"},
			"Sensors", "hallway : quiet",
		},
		{"an empty title", []string{":", "Just", "the", "body"}, "", "Just the body"},
		{"nothing at all", nil, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, body := split(tt.in)
			if title != tt.title || body != tt.body {
				t.Errorf("split(%v) = %q, %q; want %q, %q", tt.in, title, body, tt.title, tt.body)
			}
		})
	}
}

// A card with nothing on it says nothing, which is the one thing the action refuses too.
func TestAMessageWithNoBodyIsRefused(t *testing.T) {
	for _, args := range [][]string{
		{"info", "5", "Title", ":"},
		{"info", "5", "  "},
		{"info", "5", ":"},
	} {
		if err := say(args); err == nil {
			t.Errorf("%v was accepted", args)
		}
	}
}

func TestTheArgumentsAreChecked(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"nothing", nil},
		{"no body", []string{"info", "5"}},
		{"a tone that is not one", []string{"urgent", "5", "Something"}},
		{"seconds that are not a number", []string{"info", "soon", "Something"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := say(tt.args); err == nil {
				t.Error("accepted")
			}
		})
	}
}
