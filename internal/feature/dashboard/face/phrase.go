package face

// The time in English, kept apart from the face that draws it: this is language and that is
// layout, and a second language would land here and nowhere else.

var names = [...]string{
	"twelve", "one", "two", "three", "four", "five",
	"six", "seven", "eight", "nine", "ten", "eleven",
}

var counts = map[int]string{
	5: "five", 10: "ten", 20: "twenty", 25: "twenty-five",
}

// say is the time in words, already broken into the lines it wants to be read in.
//
// Broken here rather than wrapped at the drawing, because where the phrase divides is part of how
// it reads: "half past" belongs on its own line and "o'clock" belongs under the hour it follows.
// Always two lines, so the face does not resize itself between one phrase and the next.
//
// Rounded to five minutes, because "twenty three minutes past six" is not how anybody answers.
func Say(hour, minute int) []string {
	// Rounding to five can carry: 58 rounds to 60, which is the next hour rather than the same one
	// at zero. Wrapping the minutes without the hour is how 6:58 came out as six o'clock.
	near := (minute + 2) / 5 * 5
	if near >= 60 {
		hour++
		near = 0
	}

	// Past the half the phrase turns around, and the hour it names is the next one.
	to := near >= 35
	if to {
		hour++
		near = 60 - near
	}
	name := names[((hour%12)+12)%12]

	switch {
	case near == 0:
		return []string{name, "o'clock"}
	case near == 30:
		return []string{"half past", name}
	case near == 15 && !to:
		return []string{"quarter past", name}
	case near == 15 && to:
		return []string{"quarter to", name}
	}

	count, ok := counts[near]
	if !ok {
		// A minute with no word for it means the rounding above changed and this did not. Saying
		// the hour is wrong by under three minutes, which beats saying nothing.
		return []string{name, "o'clock"}
	}

	if to {
		return []string{count + " to", name}
	}
	return []string{count + " past", name}
}
