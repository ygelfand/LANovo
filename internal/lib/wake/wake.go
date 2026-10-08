package wake

import (
	"sync"

	"github.com/ygelfand/LANovo/internal/layout"

	sharedwake "github.com/ygelfand/libcountertop/pkg/inference/wake"
)

var get = sync.OnceValue(
	func() *sharedwake.Library { return sharedwake.NewLibrary(layout.ModelDir) },
)

func Lib() *sharedwake.Library { return get() }
