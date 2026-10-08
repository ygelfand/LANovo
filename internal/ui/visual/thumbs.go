package visual

import (
	"embed"

	sharedvisual "github.com/ygelfand/libcountertop/pkg/display/visual"
)

//go:embed thumbs/*.jpg
var thumbFiles embed.FS

var thumbs = sharedvisual.NewThumbs(thumbFiles, "thumbs")

func Thumbs() *sharedvisual.Thumbs { return thumbs }
