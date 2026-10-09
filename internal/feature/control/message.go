package control

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ygelfand/libcountertop/pkg/settings/schema"

	sharedmessage "github.com/ygelfand/libcountertop/pkg/display/message"

	"github.com/ygelfand/LANovo/internal/feature/message"
)

func say(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("want TONE SECONDS TITLE : BODY")
	}

	tone := schema.Tone(args[0])
	if !slices.Contains(schema.Tones(), tone) {
		return fmt.Errorf("no such tone %q, want one of %s", args[0], tones())
	}

	secs, err := strconv.Atoi(args[1])
	if err != nil {
		return fmt.Errorf("seconds: %w", err)
	}

	title, body := split(args[2:])
	if body == "" {
		return fmt.Errorf("the message has no body")
	}

	message.Get().
		Show(sharedmessage.Message{Title: title, Body: body, Tone: tone}, sharedmessage.Hold(secs))
	return nil
}

func split(args []string) (title, body string) {
	for i, a := range args {
		if a == ":" {
			return join(args[:i]), join(args[i+1:])
		}
	}
	return "", join(args)
}

func join(words []string) string { return strings.TrimSpace(strings.Join(words, " ")) }

func tones() string {
	out := make([]string, 0, len(schema.Tones()))
	for _, t := range schema.Tones() {
		out = append(out, string(t))
	}
	return strings.Join(out, ", ")
}
