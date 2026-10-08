package layout

import "testing"

func TestMAC(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"what sysfs holds", "00:f4:8d:47:69:21\n", "00:f4:8d:47:69:21"},
		{"upper case is normalized", "00:F4:8D:47:69:21", "00:f4:8d:47:69:21"},
		{"separators are not required", "00f48d476921", "00:f4:8d:47:69:21"},
		{"an unset address identifies nothing", "00:00:00:00:00:00", ""},
		{"too short", "00:f4:8d", ""},
		{"too long", "00:f4:8d:47:69:21:99", ""},
		{"an error message is not an address", "cat: /sys/...: No such file", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MAC(tt.raw); got != tt.want {
				t.Errorf("MAC(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNameFromMAC(t *testing.T) {
	tests := []struct {
		name string
		mac  string
		want string
	}{
		{"the last three octets", "00:f4:8d:47:69:21", DefaultName + " 476921"},
		{"already bare hex", "00f48d476921", DefaultName + " 476921"},
		{"nothing to key on falls back", "", DefaultName},
		{"too short to be unique falls back", "ab:cd", DefaultName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NameFromMAC(tt.mac); got != tt.want {
				t.Errorf("NameFromMAC(%q) = %q, want %q", tt.mac, got, tt.want)
			}
		})
	}
}

func TestSlug(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"a plain name", "Kitchen Display", "kitchen-display"},
		{"the default", DefaultName + " 476921", "smart-display-476921"},
		{"runs of punctuation collapse to one dash", "Kitchen  --  Display", "kitchen-display"},
		{"a trailing separator is dropped", "Kitchen!", "kitchen"},
		{"a leading separator never starts one", "  !Kitchen", "kitchen"},
		{"accents are not letters here", "Café", "caf"},
		{"digits are kept", "Display 2", "display-2"},
		{"nothing usable", "!!!", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Slug(tt.in); got != tt.want {
				t.Errorf("Slug(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSuggestedNameSlugFits(t *testing.T) {
	slug := Slug(NameFromMAC("00:f4:8d:47:69:21"))

	if slug == "" {
		t.Fatal("the suggested name slugifies to nothing")
	}
	if len(slug) > MaxNodeName {
		t.Errorf("the suggested name becomes %q, %d characters, and the limit is %d",
			slug, len(slug), MaxNodeName)
	}
}
