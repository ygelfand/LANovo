package main

import (
	"fmt"
	"strconv"
	"strings"
)

// SVG path data: separators are optional where a sign or decimal point ends a number.

type scanner struct {
	s string
	i int
}

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

func (p *scanner) done() bool {
	p.skip()
	return p.i >= len(p.s)
}

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
