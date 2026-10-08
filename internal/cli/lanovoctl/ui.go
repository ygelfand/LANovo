package lanovoctl

import (
	"context"
	"io"

	"charm.land/lipgloss/v2"
	"github.com/ygelfand/libcountertop/pkg/host/adb"
	"github.com/ygelfand/libcountertop/pkg/host/prompt"
)

var (
	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	styleDone   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	styleFail   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	styleDetail = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func connect(ctx context.Context, out io.Writer) (*adb.Device, error) {
	if err := adb.Require(); err != nil {
		return nil, err
	}

	target, err := prompt.Serial(ctx, out, serial, adb.List, "")
	if err != nil {
		return nil, err
	}
	return adb.Attach(target)
}

func mark(ok bool) string {
	if ok {
		return styleDone.Render("ok")
	}
	return styleFail.Render("no")
}
