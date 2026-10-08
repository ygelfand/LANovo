package control

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ygelfand/LANovo/internal/hardware/speaker"
	"github.com/ygelfand/LANovo/internal/lib/alsa"
)

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

	if c, err := m.Find(name); err == nil {
		return reading(m, c)
	}
	return matching(m, name)
}

func either(args []string) (name, value string, setting bool) {
	at := slices.Index(args, "=")
	if at < 0 {
		return strings.Join(args, " "), "", false
	}
	return strings.Join(args[:at], " "), strings.Join(args[at+1:], " "), true
}

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
