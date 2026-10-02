package videoplayer

import (
	"time"

	"github.com/ygelfand/LANovo/internal/feature/media"
	"github.com/ygelfand/LANovo/internal/ui/theme"
)

type Controls interface {
	media.Source
	Seek(to time.Duration)
	CanSeek() bool
}

type Mark struct {
	From, To time.Duration
	Color    theme.Color
}

type Marked interface {
	Marks() []Mark
}

const linger = 4 * time.Second
