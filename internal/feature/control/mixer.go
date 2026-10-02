package control

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/lib/alsa"
)

// mixer reads and sets a control on the sound card, for finding out what a route does before any
// of it is written down as driver code.
//
// The card is the one thing about this device whose behaviour is not in the source: the mixer XML
// describes several boards and the HAL replays only part of it, so what a control actually does is
// a question the card answers and nothing else can. This is how to ask it.
//
// Nothing here is saved. A control set this way goes back to whatever the driver sets the next
// time lanovod starts, which is what makes it safe to try things: the way out of a wrong answer is
// a restart.
//
// Names have spaces in them, so a value is separated by an equals rather than by position:
//
//	mixer AUDIO_REF_EC_UL1 MUX
//	mixer AUDIO_REF_EC_UL1 MUX = QUAT_MI2S_RX
//	mixer ref
func mixer(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("mixer: which control")
	}

	name, value, setting := either(args)
	if name == "" {
		return "", fmt.Errorf("mixer: which control")
	}

	m, err := alsa.OpenMixer(speaker.Card)
	if err != nil {
		return "", fmt.Errorf("mixer: %w", err)
	}
	defer m.Close()

	if setting {
		return write(m, name, value)
	}

	// An exact name reads that control; anything else is somebody looking for one.
	if c, err := m.Find(name); err == nil {
		return reading(m, c)
	}
	return matching(m, name)
}

// either takes the name and the value from the two sides of an equals, and says whether there
// was one.
func either(args []string) (name, value string, setting bool) {
	at := slices.Index(args, "=")
	if at < 0 {
		return strings.Join(args, " "), "", false
	}
	return strings.Join(args[:at], " "), strings.Join(args[at+1:], " "), true
}

// write sets a control, by item name for an enumerated one and by number for the rest.
func write(m *alsa.Mixer, name, value string) (string, error) {
	c, err := m.Find(name)
	if err != nil {
		return "", fmt.Errorf("mixer: %w", err)
	}

	if c.Type == alsa.TypeEnumerated {
		if !slices.Contains(c.Items, value) {
			return "", fmt.Errorf("mixer: %q takes one of %s", name, strings.Join(c.Items, ", "))
		}
		if err := m.SetEnum(name, value); err != nil {
			return "", fmt.Errorf("mixer: %w", err)
		}
		return reading(m, c)
	}

	v, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return "", fmt.Errorf("mixer: %q takes a number, not %q", name, value)
	}
	if err := m.SetInt(name, uint32(v)); err != nil {
		return "", fmt.Errorf("mixer: %w", err)
	}
	return reading(m, c)
}

// reading is a control's value, with what else it could be when that is a short list: a mux says
// nothing useful without the routes it can take.
func reading(m *alsa.Mixer, c alsa.Control) (string, error) {
	v, err := m.Get(c)
	if err != nil {
		return "", fmt.Errorf("mixer: %w", err)
	}

	if c.Type == alsa.TypeEnumerated {
		var now []string
		for _, at := range v {
			if int(at) < len(c.Items) {
				now = append(now, c.Items[at])
				continue
			}
			now = append(now, strconv.FormatUint(uint64(at), 10))
		}
		return fmt.Sprintf("%s = %s\n  of %s",
			c.Name, strings.Join(now, " "), strings.Join(c.Items, ", ")), nil
	}

	said := make([]string, 0, len(v))
	for _, at := range v {
		said = append(said, strconv.FormatUint(uint64(at), 10))
	}

	out := fmt.Sprintf("%s = %s", c.Name, strings.Join(said, " "))
	if c.Max > c.Min {
		out += fmt.Sprintf("\n  of %d to %d", c.Min, c.Max)
	}
	return out, nil
}

// matching is every control whose name contains what was asked for, for finding the one that does
// a thing when its name is only half remembered.
func matching(m *alsa.Mixer, want string) (string, error) {
	found, err := speaker.Controls(want)
	if err != nil {
		return "", err
	}
	if len(found) == 0 {
		return "", fmt.Errorf("mixer: no control matches %q", want)
	}
	return strings.Join(found, "\n"), nil
}
