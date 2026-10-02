// Package setting is how a thing the user can change is described once and shown everywhere.
//
// A feature declares a table: one row per setting, saying what it is called, what kind of control it
// wants, what values it will take and how to move one on and off the feature's own configuration.
// The panel, the control socket and Home Assistant all render from that description, so adding a
// setting costs a row rather than edits in three places.
//
// Generic over the configuration it writes, so a table carries no other feature's types with it.
package setting

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/ygelfand/LANovo/internal/lib/say"
)

// On and Off are how a boolean setting is written: in the saved state, at the control socket, and
// in a row's Read. Nothing outside this package compares against the words.
const (
	On  = "on"
	Off = "off"
)

// OnOff is a boolean as a setting is written.
func OnOff(b bool) string {
	if b {
		return On
	}
	return Off
}

// Boolean reads one back. Empty is on, which is what a bare toggle name at the control socket means.
func Boolean(v string) (bool, bool) {
	switch strings.ToLower(v) {
	case On, "true", "yes", "1", "":
		return true, true
	case Off, "false", "no", "0":
		return false, true
	}
	return false, false
}

// Kind is what sort of control a setting wants.
type Kind int

const (
	// Toggle takes "mirror" or "nomirror" as well as name=on.
	Toggle Kind = iota

	Number

	Choice

	// Pair is WIDTHxHEIGHT.
	Pair

	// Triple is three comma-separated numbers.
	Triple

	List
)

func (k Kind) String() string {
	switch k {
	case Toggle:
		return "toggle"
	case Number:
		return "number"
	case Choice:
		return "choice"
	case Pair:
		return "pair"
	case Triple:
		return "triple"
	case List:
		return "list"
	}
	return "unknown"
}

// Group is the section a setting belongs under.
type Group string

// Scale maps a Number's range onto nought to a hundred. Doubling gives equal stops, which is what
// light wants; a linear gain knob puts 32x at halfway.
type Scale int

const (
	Linear Scale = iota
	Doubling
)

type Option struct {
	Value string

	// Label is what to show when the text is the same in every language, like a size or an angle.
	Label string

	// Key is the last part of the message to show instead, for an option that is a word.
	Key string
}

// Setting is one thing that can be changed, and how to move it on and off a T.
type Setting[T any] struct {
	// Name is the identifier everywhere: the config key, the control socket's argument, and the
	// message the panel shows it under.
	Name string

	Kind  Kind
	Group Group

	// ID is what Home Assistant knows the control by, where that is not <domain>_<name>. Changing
	// one orphans the entity it used to name, so a row that already shipped keeps its own.
	ID string

	// Icon is the mdi name shown beside it.
	Icon string

	// Slider asks for a Number to be dragged rather than typed. A range somebody reads as a
	// proportion wants one; a raw sensor value does not.
	Slider bool

	// Min and Max bound a Number; both zero is unbounded. OneX is the value meaning 1x.
	Min, Max int
	Unit     string
	OneX     int
	Scale    Scale

	// Enables says this Toggle is what turns its whole group on, so a section that is off can say
	// so rather than reporting a setting nothing is using.
	Enables bool

	Options []Option

	// Read gives the current value in the text Write takes. Write refuses what it will not accept.
	Read  func(*T) string
	Write func(*T, string) error

	// Idle says the setting is showing but has nothing to do, because something else is driving
	// what it sets. Nil is a setting that always applies.
	Idle func(*T) bool

	// IdleAs is what the row says instead of its value while it is idle, naming what has taken
	// it over.
	IdleAs string

	// Sums is what this setting contributes to its group's line on a menu, as a key resolved under
	// the table's domain. Nil is a setting that has nothing to say there.
	Sums func(*T) string

	// domain is the table this row came from, for the messages. Set when the table is built.
	domain string
}

// Table is a feature's settings, and the name its messages are keyed under.
type Table[T any] struct {
	// Domain is the first part of every message: <domain>.setting.<name>, <domain>.group.<group>,
	// <domain>.option.<name>.<key> and <domain>.sums.<key>.
	Domain string

	rows   []Setting[T]
	groups []Group
	alias  map[string]string
}

// NewTable stamps each row with the domain so a row knows its own messages.
func NewTable[T any](domain string, groups []Group, rows []Setting[T]) *Table[T] {
	for i := range rows {
		rows[i].domain = domain
	}
	return &Table[T]{Domain: domain, rows: rows, groups: groups}
}

// Alias records the spellings a row used to answer to.
func (t *Table[T]) Alias(from, to string) *Table[T] {
	if t.alias == nil {
		t.alias = map[string]string{}
	}
	t.alias[from] = to
	return t
}

func (t *Table[T]) Rows() []Setting[T] { return t.rows }

// Groups is the display order.
func (t *Table[T]) Groups() []Group { return t.groups }

// In is the rows of one group, in the table's order.
func (t *Table[T]) In(g Group) []Setting[T] {
	var out []Setting[T]
	for _, s := range t.rows {
		if s.Group == g {
			out = append(out, s)
		}
	}
	return out
}

// Find is the row of that name, following any alias.
func (t *Table[T]) Find(name string) (Setting[T], bool) {
	if to, ok := t.alias[name]; ok {
		name = to
	}
	for _, s := range t.rows {
		if s.Name == name {
			return s, true
		}
	}
	return Setting[T]{}, false
}

// Row is the setting of that name, for a Write wording a refusal from its own bounds.
func (t *Table[T]) Row(name string) Setting[T] {
	s, ok := t.Find(name)
	if !ok {
		panic(t.Domain + ": no setting called " + name)
	}
	return s
}

// Title is what a section is called. The table carries only identifiers, so there is one place a
// name lives and it is the one translators are given.
func (t *Table[T]) Title(g Group) string {
	return say.T(t.Domain + ".group." + strings.ToLower(strings.ReplaceAll(string(g), " ", "")))
}

// Sums is a group's line on a menu, as the first row with something to say.
func (t *Table[T]) Sums(g Group, c *T) string {
	in := t.In(g)

	for _, s := range in {
		if s.Enables && !s.On(c) {
			return say.T(t.Domain + ".disabled")
		}
	}
	for _, s := range in {
		if s.Sums != nil {
			return say.T(t.Domain + ".sums." + s.Sums(c))
		}
	}
	for _, s := range in {
		if s.Kind == Choice {
			return s.Shows(c)
		}
	}
	return ""
}

// Apply writes one setting by name.
func (t *Table[T]) Apply(c *T, name, value string) error {
	s, ok := t.Find(name)
	if !ok {
		return fmt.Errorf("%s: nothing called %q can be set", t.Domain, name)
	}
	return s.Put(c, value)
}

// Configured is a starting point with saved settings over it. Unknown names are skipped and values
// that no longer apply are returned.
func (t *Table[T]) Configured(start T, saved map[string]string) (T, []error) {
	var bad []error
	for name, v := range saved {
		s, ok := t.Find(name)
		if !ok {
			continue
		}
		if err := s.Put(&start, v); err != nil {
			bad = append(bad, err)
		}
	}
	return start, bad
}

// Arg applies "name=value", a bare toggle name, or "noname". It reports whether the argument was
// one of ours.
func (t *Table[T]) Arg(c *T, arg string) (bool, error) {
	if name, value, ok := strings.Cut(arg, "="); ok {
		s, known := t.Find(name)
		if !known {
			return false, nil
		}
		return true, s.Put(c, value)
	}

	if s, ok := t.Find(arg); ok && s.Kind == Toggle {
		return true, s.Put(c, On)
	}
	if off, ok := strings.CutPrefix(arg, "no"); ok {
		if s, known := t.Find(off); known && s.Kind == Toggle {
			return true, s.Put(c, Off)
		}
	}
	return false, nil
}

// Object is the identifier Home Assistant keys the control on.
func (s Setting[T]) Object(domain string) string {
	if s.ID != "" {
		return s.ID
	}
	return domain + "_" + strings.ReplaceAll(s.Name, ".", "_")
}

// Title is what this setting is called on a panel, under a heading that says which section it is in.
func (s Setting[T]) Title() string { return say.T(s.domain + ".setting." + s.Name) }

// Named is what to call it where there is no heading to give it context, which is how Home Assistant
// shows one. A row only needs the longer form when its short one reads as a question: Auto, of what.
func (s Setting[T]) Named() string {
	if at := s.domain + ".entity." + s.Name; !say.Missing(at) {
		return say.T(at)
	}
	return s.Title()
}

// Label is one option as a person reads it: a word that is translated, or a size or an angle that
// is the same in every language.
func (s Setting[T]) Label(o Option) string {
	if o.Key != "" {
		return say.T(s.domain + ".option." + s.Name + "." + o.Key)
	}
	return o.Label
}

// Values is every option's stored value, for a listing that shows what may be typed.
func (s Setting[T]) Values() []string {
	out := make([]string, 0, len(s.Options))
	for _, o := range s.Options {
		out = append(out, o.Value)
	}
	return out
}

// Labels is every option in order, for a control that offers a list.
func (s Setting[T]) Labels() []string {
	out := make([]string, 0, len(s.Options))
	for _, o := range s.Options {
		out = append(out, s.Label(o))
	}
	return out
}

// Shows is what the setting says on the right: what has taken it over, or its value as a person
// reads it.
func (s Setting[T]) Shows(c *T) string {
	if s.IdleAs != "" && s.Idle != nil && s.Idle(c) {
		return s.IdleAs
	}
	return s.LabelOf(s.Read(c))
}

// LabelOf is a value as a person reads it, falling back to the value itself.
func (s Setting[T]) LabelOf(value string) string {
	for _, o := range s.Options {
		if o.Value == value {
			return s.Label(o)
		}
	}
	return value
}

// ValueOf is what is behind a label, or empty for one no option carries.
func (s Setting[T]) ValueOf(label string) string {
	for _, o := range s.Options {
		if s.Label(o) == label {
			return o.Value
		}
	}
	return ""
}

// On reads a Toggle, so nothing outside compares against the word.
func (s Setting[T]) On(c *T) bool { return s.Read(c) == On }

// Set writes a Toggle from a bool.
func (s Setting[T]) Set(c *T, on bool) error { return s.Write(c, OnOff(on)) }

// Level reads a Number, and nought for anything that is not one.
func (s Setting[T]) Level(c *T) int {
	n, err := strconv.Atoi(s.Read(c))
	if err != nil {
		return 0
	}
	return n
}

// Dim says the setting is showing but something else is driving it.
func (s Setting[T]) Dim(c *T) bool { return s.Idle != nil && s.Idle(c) }

// Bad is the error for a value a setting will not take.
func (s Setting[T]) Bad(v string, why string) error {
	if s.Kind == Choice && len(s.Options) > 0 {
		names := make([]string, 0, len(s.Options))
		for _, o := range s.Options {
			names = append(names, o.Value)
		}
		return fmt.Errorf("%s: %s takes %s, not %q", s.domain, s.Name, strings.Join(names, " or "), v)
	}
	if why == "" {
		return fmt.Errorf("%s: %q is not a %s for %s", s.domain, v, s.Kind, s.Name)
	}
	return fmt.Errorf("%s: %q is not %s for %s", s.domain, v, why, s.Name)
}

// Range is a Number's bounds as text.
func (s Setting[T]) Range() string {
	if s.Kind != Number {
		return ""
	}
	out := fmt.Sprintf("%d to %d", s.Min, s.Max)
	if s.Unit != "" {
		out += " " + s.Unit
	}
	if s.OneX != 0 {
		out += fmt.Sprintf(", %d is 1x", s.OneX)
	}
	return out
}

// Percent is where a raw value sits on nought to a hundred. Out of range clamps.
func (s Setting[T]) Percent(raw int) int {
	if s.Kind != Number || s.Max <= s.Min {
		return 0
	}
	raw = min(max(raw, s.Min), s.Max)

	if s.Scale == Doubling {
		lo, hi, at := math.Log2(float64(s.Min)), math.Log2(float64(s.Max)), math.Log2(float64(raw))
		return int(math.Round((at - lo) / (hi - lo) * 100))
	}
	return int(math.Round(float64(raw-s.Min) / float64(s.Max-s.Min) * 100))
}

func (s Setting[T]) Raw(pct int) int {
	if s.Kind != Number || s.Max <= s.Min {
		return 0
	}
	pct = min(max(pct, 0), 100)

	if s.Scale == Doubling {
		lo, hi := math.Log2(float64(s.Min)), math.Log2(float64(s.Max))
		return int(math.Round(math.Pow(2, lo+(hi-lo)*float64(pct)/100)))
	}
	return s.Min + int(math.Round(float64(s.Max-s.Min)*float64(pct)/100))
}

// Neutral is where a default sits on this setting's knob.
func (s Setting[T]) Neutral(defaults T) int {
	if s.Kind != Number {
		return 0
	}
	return s.Percent(s.Level(&defaults))
}

// Number reads a value and checks it against the bounds.
func (s Setting[T]) Number(v string) (int, error) {
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, s.Bad(v, "a number")
	}
	if s.Min != 0 || s.Max != 0 {
		if n < s.Min || n > s.Max {
			return 0, s.Bad(v, "within "+s.Range())
		}
	}
	return n, nil
}

// Put applies a value. An empty one leaves the setting alone, except on a Toggle where it means on.
func (s Setting[T]) Put(c *T, v string) error {
	if v == "" && s.Kind != Toggle {
		return nil
	}
	return s.Write(c, v)
}
