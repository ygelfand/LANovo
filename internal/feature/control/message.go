package control

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/ygelfand/LANovo/internal/feature/message"
	sharedmessage "github.com/ygelfand/libcountertop/pkg/display/message"
)

// say puts a message on the screen, which is otherwise only reachable by Home Assistant calling an
// action. A card that can only be brought up by the thing it is meant to be seen from cannot be
// looked at while it is being built.
//
//	message TONE SECONDS TITLE : BODY
//
// Both halves are prose with spaces in, and quoting rules in a protocol whose point is being typed
// by hand are a worse trade than one reserved word. A message with no separator is all body, which
// is the common case.
func say(args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("want TONE SECONDS TITLE : BODY")
	}

	// Refused rather than treated as ordinary, which is what the action does with an unrecognized
	// tone. A message from Home Assistant is worth showing however it was labeled; one typed here
	// is being typed to look at the labeling.
	tone := sharedmessage.Tone(args[0])
	if !slices.Contains(sharedmessage.Tones(), tone) {
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

	message.Get().Show(sharedmessage.Message{Title: title, Body: body, Tone: tone}, sharedmessage.Hold(secs))
	return nil
}

// split takes the title off the front, at a word that is nothing but a colon.
//
// A whole word rather than a character, and a colon rather than a pipe: this gets typed at a shell
// prompt, where a pipe is a pipe. Prose attaches a colon to the word before it, so a lone one is
// never part of what somebody meant to say.
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
	out := make([]string, 0, len(sharedmessage.Tones()))
	for _, t := range sharedmessage.Tones() {
		out = append(out, string(t))
	}
	return strings.Join(out, ", ")
}
