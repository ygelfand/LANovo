package dashboard

import (
	sharedlib "github.com/ygelfand/libcountertop/pkg/display/dashboard"

	"github.com/ygelfand/LANovo/internal/feature/shell"
)

var strip = sharedlib.NewTabs(func() { shell.Get().Redraw() })

func Tabs() *sharedlib.Tabs { return strip }
