package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Reading the d attribute of an svg path.
//
// The grammar is looser than it looks. Separators are optional wherever a sign or a decimal point
// already says a number has ended, so "10-5" is two numbers and "1.5.5" is two more. A command
// letter may be left out when it repeats, and after a moveto the repeat is a lineto rather than
// another moveto.

type scanner struct {
	s string
	i int
}

// skip steps over whitespace and commas, which separate nothing that is not already separated.
func (p *scanner) skip() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\r', '\n', ',':
			p.i++
		default:
			return
		}
	}
}

// done reports whether the whole attribute has been read.
func (p *scanner) done() bool {
	p.skip()
	return p.i >= len(p.s)
}

// command is the next letter, or zero where a number comes next and the last command repeats.
func (p *scanner) command() byte {
	p.skip()
	if p.i >= len(p.s) {
		return 0
	}

	c := p.s[p.i]
	if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' {
		p.i++
		return c
	}
	return 0
}

// number reads one, however little punctuation separates it from the last.
func (p *scanner) number() (float32, error) {
	p.skip()
	start := p.i

	if p.i < len(p.s) && (p.s[p.i] == '+' || p.s[p.i] == '-') {
		p.i++
	}

	digits := func() {
		for p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			p.i++
		}
	}

	digits()
	if p.i < len(p.s) && p.s[p.i] == '.' {
		p.i++
		digits()
	}

	// An exponent only counts when a sign or a digit follows, or "1e" would swallow the e of a
	// command that happens to sit next to it.
	if p.i < len(p.s) && (p.s[p.i] == 'e' || p.s[p.i] == 'E') {
		at := p.i
		p.i++
		if p.i < len(p.s) && (p.s[p.i] == '+' || p.s[p.i] == '-') {
			p.i++
		}
		if p.i < len(p.s) && p.s[p.i] >= '0' && p.s[p.i] <= '9' {
			digits()
		} else {
			p.i = at
		}
	}

	if p.i == start {
		return 0, fmt.Errorf("icon: no number at %q", rest(p.s, p.i))
	}

	f, err := strconv.ParseFloat(p.s[start:p.i], 32)
	if err != nil {
		return 0, fmt.Errorf("icon: %q is not a number", p.s[start:p.i])
	}
	return float32(f), nil
}

// flag is the one bit an arc uses for its two choices, which may be written without a separator.
func (p *scanner) flag() (bool, error) {
	p.skip()
	if p.i >= len(p.s) {
		return false, fmt.Errorf("icon: an arc ended before its flags")
	}

	switch p.s[p.i] {
	case '0':
		p.i++
		return false, nil
	case '1':
		p.i++
		return true, nil
	}
	return false, fmt.Errorf("icon: %q is not an arc flag", rest(p.s, p.i))
}

func rest(s string, i int) string {
	end := min(i+12, len(s))
	return strings.TrimSpace(s[i:end])
}
