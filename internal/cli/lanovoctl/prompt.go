package lanovoctl

import (
	"context"
	"errors"
	"io"

	"charm.land/huh/v2"
)

// huh reports a canceled form as huh.ErrUserAborted.

func ask(ctx context.Context, out io.Writer, field huh.Field) error {
	err := huh.NewForm(huh.NewGroup(field)).
		WithOutput(out).
		RunWithContext(ctx)
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrCanceled
	}
	return err
}

func choose[T any](
	ctx context.Context,
	out io.Writer,
	title string,
	items []T,
	label func(T) string,
	other string,
) (T, error) {
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

func confirm(ctx context.Context, out io.Writer, title string) (bool, error) {
	var yes bool
	err := ask(ctx, out, huh.NewConfirm().Title(title).Value(&yes))
	return yes, err
}

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
