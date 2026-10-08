package lanovoctl

import (
	"context"
	"errors"
	"io"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"

	"github.com/ygelfand/LANovo/internal/host/device"
)

var ErrCanceled = errors.New("canceled")

var (
	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	styleDone   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	styleFail   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	styleDetail = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func isTerminal() bool { return term.IsTerminal(os.Stdout.Fd()) }

func connect(ctx context.Context, out io.Writer) (*device.Device, error) {
	if err := device.Require(); err != nil {
		return nil, err
	}

	target, err := resolveSerial(ctx, out, serial)
	if err != nil {
		return nil, err
	}
	return device.Connect(target)
}

func mark(ok bool) string {
	if ok {
		return styleDone.Render("ok")
	}
	return styleFail.Render("no")
}
