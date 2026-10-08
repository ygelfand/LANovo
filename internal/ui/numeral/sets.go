package numeral

var Sets = []Set{
	bebas,
	dseg,
	segment,
}

const DefaultName = "Segment"

func ByName(name string) (Set, bool) {
	for _, s := range Sets {
		if s.Name == name {
			return s, true
		}
	}
	return Set{}, false
}

func Default() Set {
	s, _ := ByName(DefaultName)
	return s
}

func Names() []string {
	out := make([]string, 0, len(Sets))
	for _, s := range Sets {
		out = append(out, s.Name)
	}
	return out
}
