package dashboard

import (
	sharedlib "github.com/ygelfand/libcountertop/pkg/display/dashboard"

	"github.com/ygelfand/LANovo/internal/feature/shell"
)

type Tab = sharedlib.Tab

var strip = sharedlib.NewTabs(func() { shell.Get().Redraw() })

func AddTabs(order int, list func() []Tab) { strip.Add(order, list) }
func Tabs() []Tab                          { return strip.List() }
func Showing() (Tab, bool)                 { return strip.Showing() }
func Show(key string)                      { strip.Show(key) }
func Clock()                               { strip.Clock() }
