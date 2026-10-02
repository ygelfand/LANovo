package numeral

// Sets is every numeral set the device offers.
//
// Hand-kept, one line each, alphabetical, meaning nothing but membership — the same shape as
// component/all. Generating a set is not the same act as shipping one: the converter writes the
// data and this decides whether anybody can pick it, so a set can be converted, looked at, and left
// out without that being an accident.
//
// Everything listed is linked in, which at a few kilobytes a set is not worth being clever about.
//
// Keep it short. A set earns its place by being told apart from the others at arm's length across a
// room; two that differ by a hairline are one set and a wasted row in the picker.
var Sets = []Set{
	bebas,
	dseg,
	segment,
}

// DefaultName is the set a face gets when it does not name one.
const DefaultName = "Segment"

// ByName finds a set. An unknown name is not one, the way an unknown theme is not one: the caller
// decides what to do about it rather than being handed something silently different.
func ByName(name string) (Set, bool) {
	for _, s := range Sets {
		if s.Name == name {
			return s, true
		}
	}
	return Set{}, false
}

// Default is the set to fall back on.
func Default() Set {
	s, _ := ByName(DefaultName)
	return s
}

// Names is every set, for offering as a setting.
func Names() []string {
	out := make([]string, 0, len(Sets))
	for _, s := range Sets {
		out = append(out, s.Name)
	}
	return out
}
