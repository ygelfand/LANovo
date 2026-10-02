package shell

import (
	"time"

	"github.com/ygelfand/LANovo/internal/ui"
	"github.com/ygelfand/LANovo/internal/ui/widget"
)

type Page struct {
	Title string

	Build func() ([]widget.Row, []func(level int))

	Tiles func() ([]widget.Cell, []func(int))

	Preview func(at ui.Rect) bool
}

func (p *Page) Covers() bool { return true }

func (p *Page) Timeout() time.Duration { return SettingsTimeout }
