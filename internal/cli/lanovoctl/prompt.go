package lanovoctl

import (
	"context"
	"errors"
	"io"

	"charm.land/huh/v2"
)

// The prompts are huh forms. A canceled form reports huh.ErrUserAborted, which is translated once
// here so every caller can treat canceling the same way.

// ask runs one field, reporting ErrCanceled when the person walked away from it.
func ask(ctx context.Context, out io.Writer, field huh.Field) error {
	err := huh.NewForm(huh.NewGroup(field)).
		WithOutput(out).
		RunWithContext(ctx)
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrCanceled
	}
	return err
}

// choose offers a list and reports what was picked. label renders one item; the zero value of T comes
// back for an extra row named by other, when one is given.
func choose[T any](ctx context.Context, out io.Writer, title string, items []T, label func(T) string, other string) (T, error) {
	var chosen T

	options := make([]huh.Option[int], 0, len(items)+1)
	for i, item := range items {
		options = append(options, huh.NewOption(label(item), i))
	}
	if other != "" {
		options = append(options, huh.NewOption(other, len(items)))
	}

	var at int
	field := huh.NewSelect[int]().Title(title).Options(options...).Value(&at)
	if err := ask(ctx, out, field); err != nil {
		return chosen, err
	}
	if at < len(items) {
		chosen = items[at]
	}
	return chosen, nil
}

// line asks for one value. secret hides what is typed and permits an empty answer.
func line(ctx context.Context, out io.Writer, title, value string, secret bool) (string, error) {
	field := huh.NewInput().Title(title).Value(&value)
	if secret {
		field = field.EchoMode(huh.EchoModePassword)
	} else {
		field = field.Validate(notEmpty)
	}

	err := ask(ctx, out, field)
	return value, err
}

// confirm asks a yes or no question, defaulting to no.
func confirm(ctx context.Context, out io.Writer, title string) (bool, error) {
	var yes bool
	err := ask(ctx, out, huh.NewConfirm().Title(title).Value(&yes))
	return yes, err
}

// confirmDefaultYes is for the question a command was already asked to do, where the prompt is a
// chance to back out rather than a decision to make.
func confirmDefaultYes(ctx context.Context, out io.Writer, title string) (bool, error) {
	yes := true
	err := ask(ctx, out, huh.NewConfirm().Title(title).Value(&yes))
	return yes, err
}

func notEmpty(s string) error {
	if s == "" {
		return errors.New("cannot be empty")
	}
	return nil
}
