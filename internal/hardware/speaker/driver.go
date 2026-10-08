package speaker

import (
	"sync"

	"github.com/ygelfand/libcountertop/pkg/audio/sound"
)

type Driver = sound.Driver[*Speaker]

var Sound = sync.OnceValue(func() *Driver { return sound.NewDriver(Get()) })
